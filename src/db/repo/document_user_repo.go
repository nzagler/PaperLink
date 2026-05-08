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
		Where("document_id = ? AND user_id = ?", documentID, userID).
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

func (r *DocumentUserRepo) GetSharesByDocumentID(documentID int) ([]entity.DocumentUser, error) {
	var shares []entity.DocumentUser
	err := r.db.
		Preload("User").
		Where("document_id = ?", documentID).
		Find(&shares).Error
	return shares, err
}

func (r *DocumentUserRepo) UpsertShare(documentID int, userID int, role entity.DocumentUserRole) (*entity.DocumentUser, error) {
	share := entity.DocumentUser{
		DocumentID: documentID,
		UserID:     userID,
		Role:       role,
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var existing entity.DocumentUser
		err := tx.Where("document_id = ? AND user_id = ?", documentID, userID).First(&existing).Error
		if err == nil {
			return tx.Model(&existing).Update("role", role).Error
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Create(&share).Error
	})
	if err != nil {
		return nil, err
	}

	return r.GetAccess(documentID, userID)
}

func (r *DocumentUserRepo) DeleteShare(documentID int, userID int) error {
	return r.db.
		Where("document_id = ? AND user_id = ?", documentID, userID).
		Delete(&entity.DocumentUser{}).Error
}
