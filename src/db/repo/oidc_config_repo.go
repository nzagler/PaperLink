package repo

import "paperlink/db/entity"

type OIDCConfigRepo struct {
	*Repository[entity.OIDCConfig]
}

func newOIDCConfigRepo() *OIDCConfigRepo {
	return &OIDCConfigRepo{NewRepository[entity.OIDCConfig]()}
}

var OIDCConfig = newOIDCConfigRepo()

func (r *OIDCConfigRepo) GetActive() (*entity.OIDCConfig, error) {
	var config entity.OIDCConfig
	err := r.db.Where("enabled = ?", true).Order("id ASC").First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *OIDCConfigRepo) GetSingleton() (*entity.OIDCConfig, error) {
	var config entity.OIDCConfig
	err := r.db.Order("id ASC").First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *OIDCConfigRepo) SaveSingleton(config *entity.OIDCConfig) error {
	config.ID = 1
	return r.db.Save(config).Error
}
