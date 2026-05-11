package entity

type User struct {
	ID           int    `gorm:"primary_key;autoIncrement"`
	Username     string `gorm:"unique;not null"`
	Password     string
	IsAdmin      bool
	TokenVersion int `gorm:"not null;default:0"`
}
