//go:build cgo && integration
// +build cgo,integration

package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"paperlink/db/entity"
	"paperlink/db/repo"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type apiResponse struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

type loginResp struct {
	AccessToken string `json:"access"`
}

func TestAuthUsernamePasswordFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	InitAuthRouter(r)

	seed := strconv.FormatInt(time.Now().UnixNano(), 10)
	initialUsername := "user_" + seed
	initialPassword := "Password123!"
	newUsername := "renamed_" + seed
	newPassword := "NewPass123!"
	inviteCode := "invite_" + seed

	invite := &entity.RegistrationInvite{
		Code:      inviteCode,
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		Uses:      1,
	}
	if err := repo.RegistrationInvite.Save(invite); err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	registerBody := map[string]string{
		"username":   initialUsername,
		"password":   initialPassword,
		"inviteCode": inviteCode,
	}
	mustStatus(t, r, http.MethodPost, "/api/v1/auth/register", registerBody, "", "", http.StatusOK)

	loginStatus, loginBody, refreshCookie := doRequest(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": initialUsername,
		"password": initialPassword,
	}, "", "")
	if loginStatus != http.StatusOK {
		t.Fatalf("expected login 200, got %d: %s", loginStatus, loginBody)
	}
	if refreshCookie == "" {
		t.Fatalf("expected refresh cookie to be set")
	}
	access := parseAccessToken(t, loginBody)

	mustStatus(t, r, http.MethodPatch, "/api/v1/auth/username", map[string]string{
		"username": newUsername,
	}, access, "", http.StatusOK)

	userByNewName, err := repo.User.GetUserByName(newUsername)
	if err != nil || userByNewName == nil {
		t.Fatalf("expected user lookup by new username to work, err=%v", err)
	}
	if _, err := repo.User.GetUserByName(initialUsername); err == nil {
		t.Fatalf("expected old username lookup to fail")
	}

	mustStatus(t, r, http.MethodPatch, "/api/v1/auth/password", map[string]string{
		"oldPassword": initialPassword,
		"newPassword": newPassword,
	}, access, "", http.StatusOK)

	// Refresh should still work after username change and return a valid access token.
	refreshStatus, refreshBody, _ := doRequest(t, r, http.MethodPost, "/api/v1/auth/refresh", nil, "", refreshCookie)
	if refreshStatus != http.StatusOK {
		t.Fatalf("expected refresh 200, got %d: %s", refreshStatus, refreshBody)
	}
	refreshedAccess := parseAccessToken(t, refreshBody)
	mustStatus(t, r, http.MethodGet, "/api/v1/auth/me", nil, refreshedAccess, "", http.StatusOK)

	// Old credentials must no longer work.
	mustStatus(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": initialUsername,
		"password": initialPassword,
	}, "", "", http.StatusUnauthorized)

	// New credentials must work.
	mustStatus(t, r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"username": newUsername,
		"password": newPassword,
	}, "", "", http.StatusOK)
}

func parseAccessToken(t *testing.T, body string) string {
	t.Helper()
	var resp apiResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("failed to parse response envelope: %v", err)
	}
	var login loginResp
	if err := json.Unmarshal(resp.Data, &login); err != nil {
		t.Fatalf("failed to parse login payload: %v", err)
	}
	if login.AccessToken == "" {
		t.Fatalf("missing access token in response")
	}
	return login.AccessToken
}

func mustStatus(
	t *testing.T,
	r http.Handler,
	method string,
	path string,
	body any,
	accessToken string,
	cookieHeader string,
	want int,
) string {
	t.Helper()
	status, respBody, _ := doRequest(t, r, method, path, body, accessToken, cookieHeader)
	if status != want {
		t.Fatalf("%s %s expected %d, got %d: %s", method, path, want, status, respBody)
	}
	return respBody
}

func doRequest(
	t *testing.T,
	r http.Handler,
	method string,
	path string,
	body any,
	accessToken string,
	cookieHeader string,
) (int, string, string) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	setCookie := rec.Header().Get("Set-Cookie")
	refreshCookie := ""
	if setCookie != "" && strings.HasPrefix(setCookie, "refresh=") {
		refreshCookie = strings.SplitN(setCookie, ";", 2)[0]
	}
	return rec.Code, rec.Body.String(), refreshCookie
}
