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
	UserID   int                       `json:"userId"`
	Username string                    `json:"username"`
	Role     entity.DocumentUserRole   `json:"role"`
	Status   entity.DocumentUserStatus `json:"status"`
}

type DocumentInviteResponse struct {
	DocumentID   int                     `json:"documentId"`
	DocumentUUID string                  `json:"documentUuid"`
	DocumentName string                  `json:"documentName"`
	Owner        string                  `json:"owner"`
	Role         entity.DocumentUserRole `json:"role"`
	UpdatedAt    string                  `json:"updatedAt"`
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

	share, err := repo.DocumentUser.UpsertInvite(doc.ID, target.ID, req.Role)
	if err != nil {
		log.Errorf("failed to invite user %d to document %s: %v", target.ID, uuid, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to invite user")
		return
	}

	routes.JSONSuccessOK(c, ShareResponse{
		UserID:   share.UserID,
		Username: share.User.Username,
		Role:     share.Role,
		Status:   share.Status,
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
			Status:   share.Status,
		})
	}

	routes.JSONSuccessOK(c, out)
}

func ListInvites(c *gin.Context) {
	userID := c.GetInt("userId")

	invites, err := repo.DocumentUser.GetPendingInvitesByUserID(userID)
	if err != nil {
		log.Errorf("failed to list document invites for user %d: %v", userID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to list invites")
		return
	}

	out := make([]DocumentInviteResponse, 0, len(invites))
	for _, invite := range invites {
		out = append(out, DocumentInviteResponse{
			DocumentID:   invite.DocumentID,
			DocumentUUID: invite.Document.UUID,
			DocumentName: invite.Document.Name,
			Owner:        invite.Document.User.Username,
			Role:         invite.Role,
			UpdatedAt:    invite.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	routes.JSONSuccessOK(c, out)
}

func AcceptInvite(c *gin.Context) {
	updateInvite(c, entity.DocumentInviteAccepted)
}

func DeclineInvite(c *gin.Context) {
	updateInvite(c, entity.DocumentInviteDeclined)
}

func updateInvite(c *gin.Context, status entity.DocumentUserStatus) {
	uuid := c.Param("id")
	userID := c.GetInt("userId")

	doc := repo.Document.GetByUUIDWithFile(uuid)
	if doc == nil {
		routes.JSONError(c, http.StatusNotFound, "document not found")
		return
	}
	if doc.UserID == userID {
		routes.JSONError(c, http.StatusBadRequest, "owner cannot respond to own invite")
		return
	}

	invite, err := repo.DocumentUser.GetInvite(doc.ID, userID)
	if err != nil || invite == nil || invite.Status != entity.DocumentInvitePending {
		routes.JSONError(c, http.StatusNotFound, "pending invite not found")
		return
	}

	if err := repo.DocumentUser.UpdateInviteStatus(doc.ID, userID, status); err != nil {
		log.Errorf("failed to update invite for document %s and user %d: %v", uuid, userID, err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update invite")
		return
	}

	routes.JSONSuccessOK(c, gin.H{"message": "ok"})
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
