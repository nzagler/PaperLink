package entity

import "time"

type DocumentUserRole string
type DocumentUserStatus string

const (
	Editor DocumentUserRole = "EDITOR"
	Viewer DocumentUserRole = "VIEWER"
)

const (
	DocumentInvitePending  DocumentUserStatus = "PENDING"
	DocumentInviteAccepted DocumentUserStatus = "ACCEPTED"
	DocumentInviteDeclined DocumentUserStatus = "DECLINED"
)

type DocumentUser struct {
	UserID     int                `gorm:"primaryKey" json:"userId"`
	DocumentID int                `gorm:"primaryKey" json:"documentId"`
	User       User               `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user"`
	Document   Document           `gorm:"foreignKey:DocumentID;constraint:OnDelete:CASCADE" json:"document"`
	Role       DocumentUserRole   `gorm:"not null" json:"role"`
	Status     DocumentUserStatus `gorm:"not null;default:ACCEPTED" json:"status"`
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
}
