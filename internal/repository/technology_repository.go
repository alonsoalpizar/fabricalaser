package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrTechnologyNotFound = errors.New("tecnología no encontrada")

type TechnologyRepository struct {
	db *gorm.DB
}

func NewTechnologyRepository() *TechnologyRepository {
	return &TechnologyRepository{
		db: database.Get(),
	}
}

// FindAll returns all active technologies
func (r *TechnologyRepository) FindAll() ([]models.Technology, error) {
	var technologies []models.Technology
	if err := r.db.Where("is_active = ?", true).Order("id").Find(&technologies).Error; err != nil {
		return nil, err
	}
	return technologies, nil
}

// FindByID finds a technology by ID
func (r *TechnologyRepository) FindByID(id uint) (*models.Technology, error) {
	var tech models.Technology
	if err := r.db.First(&tech, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechnologyNotFound
		}
		return nil, err
	}
	return &tech, nil
}

// FindByCode finds a technology by code
func (r *TechnologyRepository) FindByCode(code string) (*models.Technology, error) {
	var tech models.Technology
	if err := r.db.Where("code = ?", code).First(&tech).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechnologyNotFound
		}
		return nil, err
	}
	return &tech, nil
}

// Create creates a new technology
func (r *TechnologyRepository) Create(tech *models.Technology) error {
	return r.db.Create(tech).Error
}

// Update updates an existing technology
func (r *TechnologyRepository) Update(tech *models.Technology) error {
	return r.db.Save(tech).Error
}

// Delete soft-deletes a technology by setting is_active to false
func (r *TechnologyRepository) Delete(id uint) error {
	return r.db.Model(&models.Technology{}).Where("id = ?", id).Update("is_active", false).Error
}
