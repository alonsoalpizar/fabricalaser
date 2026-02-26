package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrSystemConfigNotFound = errors.New("configuracion no encontrada")

type SystemConfigRepository struct {
	db *gorm.DB
}

func NewSystemConfigRepository() *SystemConfigRepository {
	return &SystemConfigRepository{
		db: database.Get(),
	}
}

// FindAll returns all active system configs
func (r *SystemConfigRepository) FindAll() ([]models.SystemConfig, error) {
	var configs []models.SystemConfig
	if err := r.db.Where("is_active = ?", true).Order("category, config_key").Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// FindByID finds a system config by ID
func (r *SystemConfigRepository) FindByID(id uint) (*models.SystemConfig, error) {
	var config models.SystemConfig
	if err := r.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSystemConfigNotFound
		}
		return nil, err
	}
	return &config, nil
}

// FindByKey finds a system config by key
func (r *SystemConfigRepository) FindByKey(key string) (*models.SystemConfig, error) {
	var config models.SystemConfig
	if err := r.db.Where("config_key = ? AND is_active = ?", key, true).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSystemConfigNotFound
		}
		return nil, err
	}
	return &config, nil
}

// FindByCategory finds all system configs by category
func (r *SystemConfigRepository) FindByCategory(category string) ([]models.SystemConfig, error) {
	var configs []models.SystemConfig
	if err := r.db.Where("category = ? AND is_active = ?", category, true).Order("config_key").Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

// Create creates a new system config
func (r *SystemConfigRepository) Create(config *models.SystemConfig) error {
	return r.db.Create(config).Error
}

// Update updates an existing system config
func (r *SystemConfigRepository) Update(config *models.SystemConfig) error {
	return r.db.Save(config).Error
}

// Delete soft-deletes a system config by setting is_active to false
func (r *SystemConfigRepository) Delete(id uint) error {
	return r.db.Model(&models.SystemConfig{}).Where("id = ?", id).Update("is_active", false).Error
}
