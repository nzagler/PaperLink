package admin

import (
	"net/http"
	"strconv"

	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
)

type AdminUserResponse struct {
	ID            int    `json:"id"`
	Username      string `json:"username"`
	IsAdmin       bool   `json:"isAdmin"`
	DocumentCount int64  `json:"documentCount"`
	TotalSize     uint64 `json:"totalSize"`
	TotalPages    uint64 `json:"totalPages"`
}

type UpdateUserRoleRequest struct {
	IsAdmin bool `json:"isAdmin"`
}

func ListUsers(c *gin.Context) {
	users, err := repo.User.GetList()
	if err != nil {
		routes.JSONError(c, http.StatusInternalServerError, "failed to list users")
		return
	}

	out := make([]AdminUserResponse, 0, len(users))
	for _, user := range users {
		stats, err := repo.Document.GetUserDocumentStorageStats(user.ID)
		if err != nil {
			routes.JSONError(c, http.StatusInternalServerError, "failed to calculate user storage")
			return
		}

		out = append(out, AdminUserResponse{
			ID:            user.ID,
			Username:      user.Username,
			IsAdmin:       user.IsAdmin,
			DocumentCount: stats.DocumentCount,
			TotalSize:     stats.TotalSize,
			TotalPages:    stats.TotalPages,
		})
	}

	routes.JSONSuccessOK(c, out)
}

func UpdateUserRole(c *gin.Context) {
	targetUserID, ok := parseUserID(c)
	if !ok {
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUserID := c.GetInt("userId")
	target, err := repo.User.Get(targetUserID)
	if err != nil || target == nil {
		routes.JSONError(c, http.StatusNotFound, "user not found")
		return
	}

	if target.ID == currentUserID && target.IsAdmin && !req.IsAdmin {
		routes.JSONError(c, http.StatusBadRequest, "cannot remove your own admin role")
		return
	}

	if target.IsAdmin && !req.IsAdmin {
		adminCount, err := repo.User.CountAdmins()
		if err != nil {
			routes.JSONError(c, http.StatusInternalServerError, "failed to count admins")
			return
		}
		if adminCount <= 1 {
			routes.JSONError(c, http.StatusBadRequest, "cannot remove the last admin")
			return
		}
	}

	if err := repo.User.SetAdmin(targetUserID, req.IsAdmin); err != nil {
		log.Errorf("failed to update user %d role: %v", targetUserID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update role")
		return
	}

	routes.JSONSuccessOK(c, gin.H{"message": "ok"})
}

func InvalidateUserSessions(c *gin.Context) {
	targetUserID, ok := parseUserID(c)
	if !ok {
		return
	}

	if _, err := repo.User.Get(targetUserID); err != nil {
		routes.JSONError(c, http.StatusNotFound, "user not found")
		return
	}

	if err := repo.User.InvalidateSessions(targetUserID); err != nil {
		log.Errorf("failed to invalidate sessions for user %d: %v", targetUserID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to invalidate sessions")
		return
	}

	routes.JSONSuccessOK(c, gin.H{"message": "ok"})
}

func DeleteUser(c *gin.Context) {
	targetUserID, ok := parseUserID(c)
	if !ok {
		return
	}

	currentUserID := c.GetInt("userId")
	if targetUserID == currentUserID {
		routes.JSONError(c, http.StatusBadRequest, "cannot delete your own user")
		return
	}

	target, err := repo.User.Get(targetUserID)
	if err != nil || target == nil {
		routes.JSONError(c, http.StatusNotFound, "user not found")
		return
	}

	if target.IsAdmin {
		adminCount, err := repo.User.CountAdmins()
		if err != nil {
			routes.JSONError(c, http.StatusInternalServerError, "failed to count admins")
			return
		}
		if adminCount <= 1 {
			routes.JSONError(c, http.StatusBadRequest, "cannot delete the last admin")
			return
		}
	}

	if err := repo.Document.DeleteOrTransferDocumentsBeforeUserDelete(targetUserID); err != nil {
		log.Errorf("failed to delete or transfer documents for user %d: %v", targetUserID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to handle user documents")
		return
	}

	if err := repo.User.DeleteUser(targetUserID); err != nil {
		log.Errorf("failed to delete user %d: %v", targetUserID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to delete user")
		return
	}

	c.Status(http.StatusNoContent)
}

func parseUserID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		routes.JSONError(c, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	return id, true
}
