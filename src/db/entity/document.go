package entity

import (
	"time"

	"gorm.io/gorm"
)

type Document struct {
	ID          int    `gorm:"primaryKey" json:"id"`
	UUID        string `gorm:"uniqueIndex;not null" json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Tags []Tag `gorm:"many2many:document_tags;constraint:OnDelete:CASCADE" json:"tags"`

	UserID int  `json:"userId"`
	User   User `gorm:"constraint:OnDelete:CASCADE" json:"user"`

	FileUUID string       `json:"fileUuid"`
	File     FileDocument `gorm:"foreignKey:FileUUID;references:UUID;constraint:OnDelete:CASCADE" json:"file"`

	DirectoryID *int       `json:"directoryId"`
	Directory   *Directory `gorm:"foreignKey:DirectoryID;constraint:OnDelete:CASCADE" json:"directory"`

	UpdatedAt time.Time `json:"updatedAt"`
}

func (d *Document) BeforeSave(tx *gorm.DB) error {
	d.UpdatedAt = time.Now()
	return nil
}
