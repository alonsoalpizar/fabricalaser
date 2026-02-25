package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrMaterialNotFound = errors.New("material no encontrado")

type MaterialRepository struct {
	db *gorm.DB
}

func NewMaterialRepository() *MaterialRepository {
	return &MaterialRepository{
		db: database.Get(),
	}
}

// FindAll returns all active materials
func (r *MaterialRepository) FindAll() ([]models.Material, error) {
	var materials []models.Material
	if err := r.db.Where("is_active = ?", true).Order("factor").Find(&materials).Error; err != nil {
		return nil, err
	}
	return materials, nil
}

// FindByID finds a material by ID
func (r *MaterialRepository) FindByID(id uint) (*models.Material, error) {
	var material models.Material
	if err := r.db.First(&material, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMaterialNotFound
		}
		return nil, err
	}
	return &material, nil
}

// FindByCategory finds materials by category
func (r *MaterialRepository) FindByCategory(category string) ([]models.Material, error) {
	var materials []models.Material
	if err := r.db.Where("category = ? AND is_active = ?", category, true).Order("factor").Find(&materials).Error; err != nil {
		return nil, err
	}
	return materials, nil
}

// Create creates a new material
func (r *MaterialRepository) Create(material *models.Material) error {
	return r.db.Create(material).Error
}

// Update updates an existing material
func (r *MaterialRepository) Update(material *models.Material) error {
	return r.db.Save(material).Error
}

// Delete soft-deletes a material by setting is_active to false
func (r *MaterialRepository) Delete(id uint) error {
	return r.db.Model(&models.Material{}).Where("id = ?", id).Update("is_active", false).Error
}
