package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"paperlink/db/entity"
	"paperlink/db/repo"
	"paperlink/server/routes"
	"paperlink/util"
)

const oidcStateCookie = "paperlink_oidc_state"

type OIDCConfigRequest struct {
	IssuerURL    string `json:"issuerUrl"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Scopes       string `json:"scopes"`
	Enabled      bool   `json:"enabled"`
}

type OIDCConfigResponse struct {
	Configured bool   `json:"configured"`
	Connected  bool   `json:"connected"`
	IssuerURL  string `json:"issuerUrl"`
	ClientID   string `json:"clientId"`
	Scopes     string `json:"scopes"`
	Enabled    bool   `json:"enabled"`
}

type OIDCStatusResponse struct {
	Configured bool `json:"configured"`
	Enabled    bool `json:"enabled"`
}

type oidcFlowState struct {
	Mode      string
	UserID    int
	Nonce     string
	CreatedAt time.Time
}

type oidcClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

var oidcStates = struct {
	sync.Mutex
	values map[string]oidcFlowState
}{values: map[string]oidcFlowState{}}

func OIDCStatus(c *gin.Context) {
	config, err := repo.OIDCConfig.GetSingleton()
	if err != nil {
		routes.JSONSuccessOK(c, OIDCStatusResponse{})
		return
	}
	routes.JSONSuccessOK(c, OIDCStatusResponse{
		Configured: true,
		Enabled:    config.Enabled,
	})
}

func GetOIDCConfig(c *gin.Context) {
	userID := c.GetInt("userId")

	response := OIDCConfigResponse{}
	config, err := repo.OIDCConfig.GetSingleton()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		routes.JSONError(c, http.StatusInternalServerError, "failed to load oidc config")
		return
	}

	if config != nil {
		response.Configured = true
		response.IssuerURL = config.IssuerURL
		response.ClientID = config.ClientID
		response.Scopes = config.Scopes
		response.Enabled = config.Enabled
	}

	identity, err := repo.OIDCIdentity.GetByUserID(userID)
	if err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to load oidc identity")
		return
	}
	response.Connected = identity != nil

	routes.JSONSuccessOK(c, response)
}

func DisconnectOIDC(c *gin.Context) {
	userID := c.GetInt("userId")
	if userID == 0 {
		routes.JSONError(c, http.StatusUnauthorized, "user not authenticated")
		return
	}
	if err := repo.OIDCIdentity.DeleteByUserID(userID); err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to disconnect oidc identity")
		return
	}
	routes.JSONSuccessOK(c, gin.H{"ok": true})
}

func SaveOIDCConfig(c *gin.Context) {
	var req OIDCConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	req.IssuerURL = strings.TrimRight(strings.TrimSpace(req.IssuerURL), "/")
	req.ClientID = strings.TrimSpace(req.ClientID)
	req.ClientSecret = strings.TrimSpace(req.ClientSecret)
	req.Scopes = strings.TrimSpace(req.Scopes)
	if req.Scopes == "" {
		req.Scopes = "openid profile email"
	}
	if req.ClientSecret == "" {
		existing, err := repo.OIDCConfig.GetSingleton()
		if err == nil {
			req.ClientSecret = existing.ClientSecret
		}
	}
	if req.IssuerURL == "" || req.ClientID == "" || req.ClientSecret == "" {
		routes.JSONError(c, http.StatusBadRequest, "issuer url, client id, and client secret are required")
		return
	}
	if _, err := url.ParseRequestURI(req.IssuerURL); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "issuer url is invalid")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if _, err := oidc.NewProvider(ctx, req.IssuerURL); err != nil {
		log.Warnf("oidc discovery failed for %s: %v", req.IssuerURL, err)
		routes.JSONError(c, http.StatusBadRequest, "failed to discover oidc provider")
		return
	}

	config := entity.OIDCConfig{
		IssuerURL:    req.IssuerURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		Scopes:       req.Scopes,
		Enabled:      req.Enabled,
	}
	if err := repo.OIDCConfig.SaveSingleton(&config); err != nil {
		log.Errorf("failed to save oidc config: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to save oidc config")
		return
	}

	GetOIDCConfig(c)
}

func OIDCStart(c *gin.Context) {
	mode := c.DefaultQuery("mode", "login")
	if mode != "login" && mode != "link" {
		routes.JSONError(c, http.StatusBadRequest, "invalid oidc mode")
		return
	}

	_, _, oauthConfig, err := loadOIDCClient(c)
	if err != nil {
		routes.JSONError(c, http.StatusNotFound, err.Error())
		return
	}

	userID := 0
	if mode == "link" {
		claims, err := currentRefreshClaims(c)
		if err != nil {
			routes.JSONError(c, http.StatusUnauthorized, "sign in before linking oidc")
			return
		}
		userID = claims.UserID
	}

	state, err := randomURLToken(32)
	if err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to start oidc flow")
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to start oidc flow")
		return
	}

	oidcStates.Lock()
	oidcStates.values[state] = oidcFlowState{
		Mode:      mode,
		UserID:    userID,
		Nonce:     nonce,
		CreatedAt: time.Now(),
	}
	oidcStates.Unlock()

	c.SetCookie(oidcStateCookie, state, 600, "/api/v1/auth/oidc", "", false, true)
	c.Redirect(http.StatusFound, oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce)))
}

func OIDCCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		redirectOIDCError(c, "missing_oidc_response")
		return
	}

	cookieState, err := c.Cookie(oidcStateCookie)
	if err != nil || cookieState != state {
		redirectOIDCError(c, "invalid_oidc_state")
		return
	}
	c.SetCookie(oidcStateCookie, "", -1, "/api/v1/auth/oidc", "", false, true)

	flow, ok := consumeOIDCState(state)
	if !ok {
		redirectOIDCError(c, "expired_oidc_state")
		return
	}

	config, provider, oauthConfig, err := loadOIDCClient(c)
	if err != nil {
		redirectOIDCError(c, "oidc_not_configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		log.Warnf("oidc code exchange failed: %v", err)
		redirectOIDCError(c, "oidc_exchange_failed")
		return
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		redirectOIDCError(c, "missing_id_token")
		return
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: config.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Warnf("oidc id token verification failed: %v", err)
		redirectOIDCError(c, "invalid_id_token")
		return
	}
	if idToken.Nonce != flow.Nonce {
		redirectOIDCError(c, "invalid_oidc_nonce")
		return
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil || claims.Sub == "" {
		redirectOIDCError(c, "invalid_oidc_claims")
		return
	}

	switch flow.Mode {
	case "link":
		handleOIDCLink(c, flow, config, claims)
	default:
		handleOIDCLogin(c, config, claims)
	}
}

func handleOIDCLink(c *gin.Context, flow oidcFlowState, config *entity.OIDCConfig, claims oidcClaims) {
	if flow.UserID == 0 {
		redirectOIDCError(c, "missing_link_user")
		return
	}
	if _, err := repo.User.Get(flow.UserID); err != nil {
		redirectOIDCError(c, "link_user_not_found")
		return
	}

	existing, err := repo.OIDCIdentity.GetByIssuerSubject(config.IssuerURL, claims.Sub)
	if err == nil && existing.UserID != flow.UserID {
		redirectOIDCError(c, "oidc_identity_already_linked")
		return
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		redirectOIDCError(c, "oidc_link_failed")
		return
	}

	if err := repo.OIDCIdentity.UpsertForUser(&entity.OIDCIdentity{
		UserID:    flow.UserID,
		IssuerURL: config.IssuerURL,
		Subject:   claims.Sub,
		Email:     claims.Email,
		Name:      claims.Name,
	}); err != nil {
		log.Errorf("failed to link oidc identity: %v", err)
		redirectOIDCError(c, "oidc_link_failed")
		return
	}

	c.Redirect(http.StatusFound, "/settings?oidc=connected")
}

func handleOIDCLogin(c *gin.Context, config *entity.OIDCConfig, claims oidcClaims) {
	identity, err := repo.OIDCIdentity.GetByIssuerSubject(config.IssuerURL, claims.Sub)
	if err != nil {
		redirectOIDCError(c, "oidc_identity_not_linked")
		return
	}
	user, err := repo.User.Get(identity.UserID)
	if err != nil || user == nil {
		redirectOIDCError(c, "oidc_user_not_found")
		return
	}

	_, refresh, err := util.GenerateJWT(user.ID, user.Username)
	if err != nil {
		redirectOIDCError(c, "oidc_token_failed")
		return
	}
	setRefreshCookie(c, refresh)
	c.Redirect(http.StatusFound, "/")
}

func loadOIDCClient(c *gin.Context) (*entity.OIDCConfig, *oidc.Provider, *oauth2.Config, error) {
	config, err := repo.OIDCConfig.GetActive()
	if err != nil {
		return nil, nil, nil, errors.New("oidc is not configured")
	}
	if !config.Enabled {
		return nil, nil, nil, errors.New("oidc is disabled")
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		log.Warnf("oidc discovery failed: %v", err)
		return nil, nil, nil, errors.New("oidc discovery failed")
	}

	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  oidcRedirectURL(c),
		Scopes:       oidcScopes(config.Scopes),
	}
	return config, provider, oauthConfig, nil
}

func oidcRedirectURL(c *gin.Context) string {
	scheme := c.Request.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host + "/api/v1/auth/oidc/callback"
}

func oidcScopes(scopes string) []string {
	parts := strings.Fields(scopes)
	hasOpenID := false
	for _, scope := range parts {
		if scope == oidc.ScopeOpenID {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		parts = append([]string{oidc.ScopeOpenID}, parts...)
	}
	return parts
}

func currentRefreshClaims(c *gin.Context) (*util.UserClaims, error) {
	refreshToken, err := c.Cookie("refresh")
	if err != nil || refreshToken == "" {
		return nil, errors.New("missing refresh token")
	}
	claims, err := util.ParseJWT(refreshToken)
	if err != nil {
		return nil, err
	}
	if claims.Type != "refresh" {
		return nil, errors.New("not a refresh token")
	}
	return claims, nil
}

func consumeOIDCState(state string) (oidcFlowState, bool) {
	oidcStates.Lock()
	defer oidcStates.Unlock()

	flow, ok := oidcStates.values[state]
	if !ok || time.Since(flow.CreatedAt) > 10*time.Minute {
		delete(oidcStates.values, state)
		return oidcFlowState{}, false
	}
	delete(oidcStates.values, state)
	return flow, true
}

func randomURLToken(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func setRefreshCookie(c *gin.Context, refresh string) {
	c.SetCookie("refresh", refresh, 60*60*24*30, "/", "", false, true)
}

func redirectOIDCError(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, "/auth?oidc_error="+url.QueryEscape(code))
}
