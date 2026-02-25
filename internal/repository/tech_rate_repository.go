package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrTechRateNotFound = errors.New("tarifa no encontrada")

type TechRateRepository struct {
	db *gorm.DB
}

func NewTechRateRepository() *TechRateRepository {
	return &TechRateRepository{
		db: database.Get(),
	}
}

// FindAll returns all active tech rates with technology info
func (r *TechRateRepository) FindAll() ([]models.TechRate, error) {
	var rates []models.TechRate
	if err := r.db.Preload("Technology").Where("is_active = ?", true).Find(&rates).Error; err != nil {
		return nil, err
	}
	return rates, nil
}

// FindByID finds a tech rate by ID
func (r *TechRateRepository) FindByID(id uint) (*models.TechRate, error) {
	var rate models.TechRate
	if err := r.db.Preload("Technology").First(&rate, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechRateNotFound
		}
		return nil, err
	}
	return &rate, nil
}

// FindByTechnologyID finds a tech rate by technology ID
func (r *TechRateRepository) FindByTechnologyID(techID uint) (*models.TechRate, error) {
	var rate models.TechRate
	if err := r.db.Preload("Technology").Where("technology_id = ? AND is_active = ?", techID, true).First(&rate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechRateNotFound
		}
		return nil, err
	}
	return &rate, nil
}

// Create creates a new tech rate
func (r *TechRateRepository) Create(rate *models.TechRate) error {
	return r.db.Create(rate).Error
}

// Update updates an existing tech rate
func (r *TechRateRepository) Update(rate *models.TechRate) error {
	return r.db.Save(rate).Error
}

// Delete soft-deletes a tech rate by setting is_active to false
func (r *TechRateRepository) Delete(id uint) error {
	return r.db.Model(&models.TechRate{}).Where("id = ?", id).Update("is_active", false).Error
}
