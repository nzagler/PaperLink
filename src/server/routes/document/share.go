package document

import (
	"net/http"
	"strconv"
	"strings"

	"paperlink/db/entity"
	"paperlink/db/repo"
	"paperlink/server/routes"

	"github.com/gin-gonic/gin"
)

type ShareRequest struct {
	Username string                  `json:"username" binding:"required"`
	Role     entity.DocumentUserRole `json:"role" binding:"required"`
}

type ShareResponse struct {
	UserID   int                     `json:"userId"`
	Username string                  `json:"username"`
	Role     entity.DocumentUserRole `json:"role"`
}

func Share(c *gin.Context) {
	uuid := c.Param("id")
	if uuid == "" {
		routes.JSONError(c, http.StatusBadRequest, "invalid document uuid")
		return
	}

	var req ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		routes.JSONError(c, http.StatusBadRequest, "username is required")
		return
	}
	if req.Role != entity.Viewer && req.Role != entity.Editor {
		routes.JSONError(c, http.StatusBadRequest, "invalid role")
		return
	}

	userID := c.GetInt("userId")
	doc := repo.Document.GetByUUIDWithFile(uuid)
	if doc == nil {
		routes.JSONError(c, http.StatusNotFound, "document not found")
		return
	}
	if doc.UserID != userID {
		routes.JSONError(c, http.StatusForbidden, "only the owner can share this document")
		return
	}

	target, err := repo.User.GetUserByNameOrNil(req.Username)
	if err != nil {
		log.Errorf("failed to find user %s: %v", req.Username, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to find user")
		return
	}
	if target == nil {
		routes.JSONError(c, http.StatusNotFound, "user not found")
		return
	}
	if target.ID == userID {
		routes.JSONError(c, http.StatusBadRequest, "cannot share a document with yourself")
		return
	}

	share, err := repo.DocumentUser.UpsertShare(doc.ID, target.ID, req.Role)
	if err != nil {
		log.Errorf("failed to share document %s with user %d: %v", uuid, target.ID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to share document")
		return
	}

	routes.JSONSuccessOK(c, ShareResponse{
		UserID:   share.UserID,
		Username: share.User.Username,
		Role:     share.Role,
	})
}

func ListShares(c *gin.Context) {
	uuid := c.Param("id")
	userID := c.GetInt("userId")

	doc := repo.Document.GetByUUIDWithFile(uuid)
	if doc == nil {
		routes.JSONError(c, http.StatusNotFound, "document not found")
		return
	}
	if doc.UserID != userID {
		routes.JSONError(c, http.StatusForbidden, "only the owner can view shares")
		return
	}

	shares, err := repo.DocumentUser.GetSharesByDocumentID(doc.ID)
	if err != nil {
		log.Errorf("failed to list shares for document %s: %v", uuid, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to list shares")
		return
	}

	out := make([]ShareResponse, 0, len(shares))
	for _, share := range shares {
		out = append(out, ShareResponse{
			UserID:   share.UserID,
			Username: share.User.Username,
			Role:     share.Role,
		})
	}

	routes.JSONSuccessOK(c, out)
}

func Unshare(c *gin.Context) {
	uuid := c.Param("id")
	targetUserID, err := strconv.Atoi(c.Param("userId"))
	if err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid user id")
		return
	}
	userID := c.GetInt("userId")

	doc := repo.Document.GetByUUIDWithFile(uuid)
	if doc == nil {
		routes.JSONError(c, http.StatusNotFound, "document not found")
		return
	}
	if doc.UserID != userID {
		routes.JSONError(c, http.StatusForbidden, "only the owner can remove shares")
		return
	}

	if err := repo.DocumentUser.DeleteShare(doc.ID, targetUserID); err != nil {
		log.Errorf("failed to remove share for document %s and user %d: %v", uuid, targetUserID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to remove share")
		return
	}

	c.Status(http.StatusNoContent)
}
