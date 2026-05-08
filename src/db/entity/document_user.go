package entity

type DocumentUserRole string

const (
	Editor DocumentUserRole = "EDITOR"
	Viewer DocumentUserRole = "VIEWER"
)

type DocumentUser struct {
	UserID     int              `gorm:"primaryKey" json:"userId"`
	DocumentID int              `gorm:"primaryKey" json:"documentId"`
	User       User             `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	Document   Document         `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
	Role       DocumentUserRole `gorm:"not null" json:"role"`
}
