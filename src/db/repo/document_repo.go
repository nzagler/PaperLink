package repo

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"paperlink/db/entity"
)

type DocumentRepo struct {
	*Repository[entity.Document]
}

func newDocumentRepo() *DocumentRepo {
	return &DocumentRepo{NewRepository[entity.Document]()}
}

var Document = newDocumentRepo()

func (r *DocumentRepo) GetAnnotationsById(documentID int) ([]entity.Annotation, error) {
	var annotations []entity.Annotation
	err := r.db.Where("document_id = ?", documentID).Find(&annotations).Error
	return annotations, err
}

func (r *DocumentRepo) GetAllByUserIdWithFile(userId int) ([]entity.Document, error) {
	var result []entity.Document
	err := r.db.Preload("File").Preload("Tags").Where("user_id = ?", userId).Find(&result).Error
	return result, err
}

func (r *DocumentRepo) GetSharedByUserIdWithFile(userId int) ([]entity.Document, error) {
	var result []entity.Document
	err := r.db.
		Preload("File").
		Preload("User").
		Joins("JOIN document_users du ON du.document_id = documents.id").
		Where("du.user_id = ? AND documents.user_id <> ?", userId, userId).
		Find(&result).Error
	return result, err
}

func (r *DocumentRepo) GetByUUIDWithFile(uuid string) *entity.Document {
	var doc entity.Document
	err := r.db.Preload("File").Where("uuid = ?", uuid).First(&doc).Error
	if err != nil {
		return nil
	}
	return &doc
}

func (r *DocumentRepo) GetByUUIDWithTagsAndFile(uuid string) *entity.Document {
	var doc entity.Document
	err := r.db.
		Preload("Tags").
		Preload("File").
		Where("uuid = ?", uuid).
		First(&doc).Error
	if err != nil {
		return nil
	}
	return &doc
}

func (r *DocumentRepo) DeleteByUUID(uuid string) error {
	res := r.db.
		Where("uuid = ?", uuid).
		Delete(&entity.Document{})

	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *DocumentRepo) Filter(userID int, tags []string, search string) ([]entity.Document, error) {
	q := r.db.
		Model(&entity.Document{}).
		Preload("Tags").
		Preload("File").
		Preload("User").
		Joins("LEFT JOIN document_users du ON du.document_id = documents.id AND du.user_id = ?", userID).
		Where("(documents.user_id = ? OR du.user_id = ?)", userID, userID)

	if search != "" {
		like := "%" + strings.TrimSpace(search) + "%"
		q = q.Where("(documents.name LIKE ? OR documents.description LIKE ?)", like, like)
	}

	if len(tags) > 0 {

		q = q.
			Joins("JOIN document_tags dt ON dt.document_id = documents.id").
			Joins("JOIN tags t ON t.id = dt.tag_id").
			Where("t.name IN ?", tags).
			Group("documents.id").
			Having("COUNT(DISTINCT t.name) = ?", len(tags))
	}

	var docs []entity.Document
	err := q.Find(&docs).Error
	return docs, err
}

func (r *DocumentRepo) SaveWithAssociations(doc *entity.Document) error {
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(doc).Error
}

func (r *DocumentRepo) UpdateFieldsAndTags(doc *entity.Document, updates map[string]any, tags []entity.Tag, replaceTags bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&entity.Document{}).Where("id = ?", doc.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		if replaceTags {
			target := &entity.Document{ID: doc.ID}
			if err := tx.Model(target).Association("Tags").Replace(tags); err != nil {
				return err
			}
			doc.Tags = tags
		}

		return nil
	})
}

func (r *DocumentRepo) TouchUpdatedAt(uuid string) {
	r.db.Model(&entity.Document{}).
		Where("uuid = ?", uuid).
		Update("updated_at", time.Now())
}
