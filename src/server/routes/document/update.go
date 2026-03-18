package document

import (
	"math/rand"
	"net/http"
	"paperlink/db/entity"
	"paperlink/db/repo"
	"paperlink/server/routes"
	"strings"

	"github.com/gin-gonic/gin"
)

type UpdateRequest struct {
	UUID        string    `json:"uuid" binding:"required"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	DirectoryID *int      `json:"directoryId"`
	Tags        *[]string `json:"tags"` // nil => nicht ändern, [] => clear
}

func Update(c *gin.Context) {
	userID := c.GetInt("userId")

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		routes.JSONError(c, http.StatusBadRequest, "invalid request body")
		return
	}

	doc := repo.Document.GetByUUIDWithTagsAndFile(req.UUID)
	if doc == nil {
		routes.JSONError(c, http.StatusNotFound, "document not found")
		return
	}
	if doc.UserID != userID {
		routes.JSONError(c, http.StatusForbidden, "not authorized")
		return
	}

	if req.DirectoryID != nil {
		if *req.DirectoryID != 0 {
			dir, err := repo.Directory.Get(*req.DirectoryID)
			if err != nil || dir == nil {
				routes.JSONError(c, http.StatusBadRequest, "invalid directory")
				return
			}
			if dir.UserID != userID {
				routes.JSONError(c, http.StatusForbidden, "directory not owned by user")
				return
			}
			doc.DirectoryID = req.DirectoryID
		} else {
			doc.DirectoryID = nil
		}
	}

	if req.Name != nil {
		doc.Name = *req.Name
	}
	if req.Description != nil {
		doc.Description = *req.Description
	}

	updates := map[string]any{}
	if req.Name != nil {
		updates["name"] = doc.Name
	}
	if req.Description != nil {
		updates["description"] = doc.Description
	}
	if req.DirectoryID != nil {
		updates["directory_id"] = doc.DirectoryID
	}

	replaceTags := false
	var finalTags []entity.Tag
	if req.Tags != nil {
		replaceTags = true
		var err error
		finalTags, err = ensureTagsExist(*req.Tags)
		if err != nil {
			log.Errorf("failed processing tags: %v", err)
			routes.JSONError(c, http.StatusInternalServerError, "failed to process tags")
			return
		}
	}

	if err := repo.Document.UpdateFieldsAndTags(doc, updates, finalTags, replaceTags); err != nil {
		log.Errorf("failed to update document: %v", err)
		routes.JSONError(c, http.StatusInternalServerError, "failed to update document")
		return
	}

	routes.JSONSuccess(c, http.StatusOK, gin.H{"message": "ok"})
}

func ensureTagsExist(names []string) ([]entity.Tag, error) {
	if len(names) == 0 {
		return []entity.Tag{}, nil
	}

	dbTags, err := repo.Tag.GetList()
	if err != nil {
		return nil, err
	}

	existing := make(map[string]entity.Tag, len(dbTags))
	for _, t := range dbTags {
		existing[strings.ToLower(t.Name)] = t
	}

	finalTags := make([]entity.Tag, 0, len(names))
	seen := map[string]bool{}

	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}

		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true

		if t, ok := existing[key]; ok {
			finalTags = append(finalTags, t)
			continue
		}

		newTag := entity.Tag{
			Name:  trimmed,
			Color: entity.TagColors[rand.Intn(len(entity.TagColors))],
		}

		if err := repo.Tag.Save(&newTag); err != nil {
			return nil, err
		}

		existing[key] = newTag
		finalTags = append(finalTags, newTag)
	}

	return finalTags, nil
}
