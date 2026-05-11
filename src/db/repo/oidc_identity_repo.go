package repo

import (
	"paperlink/db/entity"

	"gorm.io/gorm"
)

type OIDCIdentityRepo struct {
	*Repository[entity.OIDCIdentity]
}

func newOIDCIdentityRepo() *OIDCIdentityRepo {
	return &OIDCIdentityRepo{NewRepository[entity.OIDCIdentity]()}
}

var OIDCIdentity = newOIDCIdentityRepo()

func (r *OIDCIdentityRepo) GetByIssuerSubject(issuerURL string, subject string) (*entity.OIDCIdentity, error) {
	var identity entity.OIDCIdentity
	err := r.db.Where("issuer_url = ? AND subject = ?", issuerURL, subject).First(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func (r *OIDCIdentityRepo) GetByUserID(userID int) (*entity.OIDCIdentity, error) {
	var identity entity.OIDCIdentity
	err := r.db.Where("user_id = ?", userID).First(&identity).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func (r *OIDCIdentityRepo) UpsertForUser(identity *entity.OIDCIdentity) error {
	var existing entity.OIDCIdentity
	err := r.db.Where("user_id = ?", identity.UserID).First(&existing).Error
	if err == nil {
		identity.ID = existing.ID
		return r.db.Save(identity).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.Save(identity).Error
}

func (r *OIDCIdentityRepo) DeleteByUserID(userID int) error {
	return r.db.Where("user_id = ?", userID).Delete(&entity.OIDCIdentity{}).Error
}
