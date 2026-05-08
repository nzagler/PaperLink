package repo

import (
	"paperlink/db/entity"

	"gorm.io/gorm"
)

type DocumentUserRepo struct {
	*Repository[entity.DocumentUser]
}

func newDocumentUserRepo() *DocumentUserRepo {
	return &DocumentUserRepo{NewRepository[entity.DocumentUser]()}
}

var DocumentUser = newDocumentUserRepo()

func (r *DocumentUserRepo) GetAccess(documentID int, userID int) (*entity.DocumentUser, error) {
	var access entity.DocumentUser
	err := r.db.
		Preload("User").
		Where("document_id = ? AND user_id = ? AND status = ?", documentID, userID, entity.DocumentInviteAccepted).
		First(&access).Error
	if err != nil {
		return nil, err
	}
	return &access, nil
}

func (r *DocumentUserRepo) HasAccess(documentID int, userID int) bool {
	access, err := r.GetAccess(documentID, userID)
	return err == nil && access != nil
}

func (r *DocumentUserRepo) HasEditorAccess(documentID int, userID int) bool {
	access, err := r.GetAccess(documentID, userID)
	return err == nil && access != nil && access.Role == entity.Editor
}

func (r *DocumentUserRepo) GetInvite(documentID int, userID int) (*entity.DocumentUser, error) {
	var invite entity.DocumentUser
	err := r.db.
		Preload("User").
		Preload("Document").
		Preload("Document.User").
		Preload("Document.File").
		Where("document_id = ? AND user_id = ?", documentID, userID).
		First(&invite).Error
	if err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *DocumentUserRepo) GetSharesByDocumentID(documentID int) ([]entity.DocumentUser, error) {
	var shares []entity.DocumentUser
	err := r.db.
		Preload("User").
		Where("document_id = ?", documentID).
		Find(&shares).Error
	return shares, err
}

func (r *DocumentUserRepo) GetSharesByDocumentIDOrdered(documentID int) ([]entity.DocumentUser, error) {
	var shares []entity.DocumentUser
	err := r.db.
		Preload("User").
		Where("document_id = ?", documentID).
		Order("created_at ASC").
		Find(&shares).Error
	return shares, err
}

func (r *DocumentUserRepo) GetPendingInvitesByUserID(userID int) ([]entity.DocumentUser, error) {
	var invites []entity.DocumentUser
	err := r.db.
		Preload("Document").
		Preload("Document.User").
		Preload("Document.File").
		Where("user_id = ? AND status = ?", userID, entity.DocumentInvitePending).
		Order("updated_at DESC").
		Find(&invites).Error
	return invites, err
}

func (r *DocumentUserRepo) UpsertInvite(documentID int, userID int, role entity.DocumentUserRole) (*entity.DocumentUser, error) {
	share := entity.DocumentUser{
		DocumentID: documentID,
		UserID:     userID,
		Role:       role,
		Status:     entity.DocumentInvitePending,
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing entity.DocumentUser
		err := tx.Where("document_id = ? AND user_id = ?", documentID, userID).First(&existing).Error
		if err == nil {
			status := entity.DocumentInvitePending
			if existing.Status == entity.DocumentInviteAccepted {
				status = entity.DocumentInviteAccepted
			}
			return tx.Model(&existing).Updates(map[string]any{
				"role":   role,
				"status": status,
			}).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(&share).Error
	})
	if err != nil {
		return nil, err
	}

	return r.GetInvite(documentID, userID)
}

func (r *DocumentUserRepo) UpdateInviteStatus(documentID int, userID int, status entity.DocumentUserStatus) error {
	return r.db.
		Model(&entity.DocumentUser{}).
		Where("document_id = ? AND user_id = ?", documentID, userID).
		Update("status", status).Error
}

func (r *DocumentUserRepo) DeleteShare(documentID int, userID int) error {
	return r.db.
		Where("document_id = ? AND user_id = ?", documentID, userID).
		Delete(&entity.DocumentUser{}).Error
}
