package entity

type OIDCConfig struct {
	ID           int    `gorm:"primary_key;autoIncrement"`
	IssuerURL    string `gorm:"not null"`
	ClientID     string `gorm:"not null"`
	ClientSecret string `gorm:"not null"`
	Scopes       string `gorm:"not null;default:openid profile email"`
	Enabled      bool   `gorm:"not null;default:false"`
}
