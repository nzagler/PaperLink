package repo

import (
	"os"
	"path/filepath"
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
		Where("du.user_id = ? AND du.status = ? AND documents.user_id <> ?", userId, entity.DocumentInviteAccepted, userId).
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
	var doc entity.Document
	if err := r.db.Preload("File").Where("uuid = ?", uuid).First(&doc).Error; err != nil {
		return err
	}

	if err := r.deleteDocumentWithFile(&doc); err != nil {
		return err
	}

	return nil
}

func (r *DocumentRepo) deleteDocumentWithFile(doc *entity.Document) error {
	filePath := doc.File.Path
	fileUUID := doc.FileUUID
	shouldRemoveFile := false

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", doc.ID).Delete(&entity.DocumentUser{}).Error; err != nil {
			return err
		}
		target := &entity.Document{ID: doc.ID}
		if err := tx.Model(target).Association("Tags").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&entity.Document{}, doc.ID).Error; err != nil {
			return err
		}

		var documentRefs int64
		if err := tx.Model(&entity.Document{}).Where("file_uuid = ?", fileUUID).Count(&documentRefs).Error; err != nil {
			return err
		}
		var d4sRefs int64
		if err := tx.Model(&entity.Digi4SchoolBook{}).Where("file_uuid = ?", fileUUID).Count(&d4sRefs).Error; err != nil {
			return err
		}
		if documentRefs > 0 || d4sRefs > 0 {
			return nil
		}

		if err := tx.Where("uuid = ?", fileUUID).Delete(&entity.FileDocument{}).Error; err != nil {
			return err
		}
		shouldRemoveFile = true
		return nil
	})
	if err != nil {
		return err
	}

	if shouldRemoveFile {
		removeDocumentFiles(filePath)
	}
	return nil
}

func removeDocumentFiles(filePath string) {
	if filePath == "" {
		return
	}

	_ = os.Remove(filePath)
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	_ = os.Remove(base + "_thumb.ptf")
}

func (r *DocumentRepo) Filter(userID int, tags []string, search string) ([]entity.Document, error) {
	q := r.db.
		Model(&entity.Document{}).
		Preload("Tags").
		Preload("File").
		Preload("User").
		Joins("LEFT JOIN document_users du ON du.document_id = documents.id AND du.user_id = ? AND du.status = ?", userID, entity.DocumentInviteAccepted).
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

type UserDocumentStorageStats struct {
	DocumentCount int64
	TotalSize     uint64
	TotalPages    uint64
}

func (r *DocumentRepo) GetUserDocumentStorageStats(userID int) (UserDocumentStorageStats, error) {
	var stats UserDocumentStorageStats
	err := r.db.
		Table("documents").
		Select("COUNT(documents.id) as document_count, COALESCE(SUM(file_documents.size), 0) as total_size, COALESCE(SUM(file_documents.pages), 0) as total_pages").
		Joins("LEFT JOIN file_documents ON file_documents.uuid = documents.file_uuid").
		Where("documents.user_id = ?", userID).
		Scan(&stats).Error
	return stats, err
}

func (r *DocumentRepo) DeleteOrTransferDocumentsBeforeUserDelete(userID int) error {
	var docs []entity.Document
	if err := r.db.Preload("File").Where("user_id = ?", userID).Find(&docs).Error; err != nil {
		return err
	}

	for _, doc := range docs {
		var shares []entity.DocumentUser
		if err := r.db.
			Where("document_id = ? AND user_id <> ? AND status = ?", doc.ID, userID, entity.DocumentInviteAccepted).
			Order("created_at ASC").
			Find(&shares).Error; err != nil {
			return err
		}

		if len(shares) == 0 {
			if err := r.deleteDocumentWithFile(&doc); err != nil {
				return err
			}
			continue
		}

		newOwnerID := shares[0].UserID
		if err := r.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&entity.Document{}).
				Where("id = ?", doc.ID).
				Updates(map[string]any{
					"user_id":      newOwnerID,
					"directory_id": nil,
				}).Error; err != nil {
				return err
			}

			return tx.Where("document_id = ? AND user_id = ?", doc.ID, newOwnerID).
				Delete(&entity.DocumentUser{}).Error
		}); err != nil {
			return err
		}
	}

	return nil
}
