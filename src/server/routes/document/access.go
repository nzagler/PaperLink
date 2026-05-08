package document

import (
	"paperlink/db/entity"
	"paperlink/db/repo"
)

func canReadDocument(doc *entity.Document, userID int) bool {
	if doc == nil {
		return false
	}
	if doc.UserID == userID {
		return true
	}
	return repo.DocumentUser.HasAccess(doc.ID, userID)
}

func canEditDocument(doc *entity.Document, userID int) bool {
	if doc == nil {
		return false
	}
	if doc.UserID == userID {
		return true
	}
	return repo.DocumentUser.HasEditorAccess(doc.ID, userID)
}
