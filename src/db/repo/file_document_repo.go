package repo

import (
	"paperlink/db/entity"
)

type FileDocumentRepo struct {
	*Repository[entity.FileDocument]
}

func newFileDocumentRepo() *FileDocumentRepo {
	return &FileDocumentRepo{NewRepository[entity.FileDocument]()}
}

var FileDocument = newFileDocumentRepo()

func (f *FileDocumentRepo) GetByUUID(uuid string) *entity.FileDocument {
	var doc entity.FileDocument
	tx := f.db.Where("uuid = ?", uuid).First(&doc)
	if tx.Error != nil {
		return nil
	}
	return &doc
}

type FileStorageStats struct {
	TotalSize  uint64
	TotalPages uint64
}

func (f *FileDocumentRepo) GetUsedStorageStats() (FileStorageStats, error) {
	var stats FileStorageStats
	err := f.db.
		Table("file_documents").
		Select("COALESCE(SUM(file_documents.size), 0) as total_size, COALESCE(SUM(file_documents.pages), 0) as total_pages").
		Where("EXISTS (SELECT 1 FROM documents WHERE documents.file_uuid = file_documents.uuid)").
		Scan(&stats).Error
	return stats, err
}
