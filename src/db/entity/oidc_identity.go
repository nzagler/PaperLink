package entity

type OIDCIdentity struct {
	ID        int    `gorm:"primary_key;autoIncrement"`
	UserID    int    `gorm:"not null;index"`
	User      User   `gorm:"constraint:OnDelete:CASCADE"`
	IssuerURL string `gorm:"not null;uniqueIndex:idx_oidc_issuer_subject"`
	Subject   string `gorm:"not null;uniqueIndex:idx_oidc_issuer_subject"`
	Email     string
	Name      string
}
