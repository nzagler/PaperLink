package repo

import (
	"paperlink/db/entity"

	"gorm.io/gorm"
)

type Digi4SchoolBookRepo struct {
	*Repository[entity.Digi4SchoolBook]
}

func newDigi4SchoolBookRepo() *Digi4SchoolBookRepo {
	return &Digi4SchoolBookRepo{NewRepository[entity.Digi4SchoolBook]()}
}

var Digi4SchoolBook = newDigi4SchoolBookRepo()

func (r *Digi4SchoolBookRepo) GetByUUID(uuid string) *entity.Digi4SchoolBook {
	var book entity.Digi4SchoolBook
	err := r.db.Where("uuid = ?", uuid).First(&book).Error
	if err != nil {
		return nil
	}
	return &book
}

func (r *Digi4SchoolBookRepo) DeleteWithUnusedFile(id int) error {
	var book entity.Digi4SchoolBook
	if err := r.db.Preload("File").Where("id = ?", id).First(&book).Error; err != nil {
		return err
	}

	filePath := book.File.Path
	fileUUID := book.FileUUID
	shouldRemoveFile := false

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&entity.Digi4SchoolBook{}, id).Error; err != nil {
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
