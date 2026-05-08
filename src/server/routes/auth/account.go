package auth

import (
	"net/http"
	"strings"

	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ChangeUsernameRequest struct {
	Username string `json:"username"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type ChangeUsernameResponse struct {
	Username string `json:"username"`
}

func ChangeUsername(c *gin.Context) {
	userID := c.GetInt("userId")
	if userID == 0 {
		routes.JSONError(c, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	newUsername := strings.TrimSpace(req.Username)
	if len(newUsername) < 3 {
		routes.JSONError(c, http.StatusBadRequest, "username must be at least 3 characters")
		return
	}

	user, err := repo.User.Get(userID)
	if err != nil || user == nil {
		routes.JSONError(c, http.StatusUnauthorized, "user not found")
		return
	}
	if user.Username == newUsername {
		routes.JSONSuccessOK(c, ChangeUsernameResponse{Username: user.Username})
		return
	}

	existing, err := repo.User.GetUserByName(newUsername)
	if err == nil && existing != nil && existing.ID != user.ID {
		routes.JSONError(c, http.StatusConflict, "username already taken")
		return
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Errorf("failed to check username availability: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update username")
		return
	}

	user.Username = newUsername
	if err := repo.User.Save(user); err != nil {
		log.Errorf("failed to save username change: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update username")
		return
	}

	routes.JSONSuccessOK(c, ChangeUsernameResponse{Username: user.Username})
}

func ChangePassword(c *gin.Context) {
	userID := c.GetInt("userId")
	if userID == 0 {
		routes.JSONError(c, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.NewPassword) < 8 {
		routes.JSONError(c, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}

	user, err := repo.User.Get(userID)
	if err != nil || user == nil {
		routes.JSONError(c, http.StatusUnauthorized, "user not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		routes.JSONError(c, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("failed to hash new password: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update password")
		return
	}

	user.Password = string(hash)
	if err := repo.User.Save(user); err != nil {
		log.Errorf("failed to save password change: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update password")
		return
	}

	routes.JSONSuccessOK(c, gin.H{"ok": true})
}
