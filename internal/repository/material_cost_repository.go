package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrMaterialCostNotFound = errors.New("costo de material no encontrado")

type MaterialCostRepository struct {
	db *gorm.DB
}

func NewMaterialCostRepository() *MaterialCostRepository {
	return &MaterialCostRepository{
		db: database.Get(),
	}
}

// FindAll returns all active material costs with material info
func (r *MaterialCostRepository) FindAll() ([]models.MaterialCost, error) {
	var costs []models.MaterialCost
	if err := r.db.Preload("Material").Where("is_active = ?", true).Find(&costs).Error; err != nil {
		return nil, err
	}
	return costs, nil
}

// FindByID finds a material cost by ID
func (r *MaterialCostRepository) FindByID(id uint) (*models.MaterialCost, error) {
	var cost models.MaterialCost
	if err := r.db.Preload("Material").First(&cost, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMaterialCostNotFound
		}
		return nil, err
	}
	return &cost, nil
}

// FindByMaterial finds all costs for a specific material
func (r *MaterialCostRepository) FindByMaterial(materialID uint) ([]models.MaterialCost, error) {
	var costs []models.MaterialCost
	if err := r.db.Preload("Material").Where("material_id = ? AND is_active = ?", materialID, true).Find(&costs).Error; err != nil {
		return nil, err
	}
	return costs, nil
}

// FindByMaterialAndThickness finds a specific cost by material and thickness
func (r *MaterialCostRepository) FindByMaterialAndThickness(materialID uint, thickness float64) (*models.MaterialCost, error) {
	var cost models.MaterialCost
	if err := r.db.Preload("Material").
		Where("material_id = ? AND thickness = ? AND is_active = ?", materialID, thickness, true).
		First(&cost).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMaterialCostNotFound
		}
		return nil, err
	}
	return &cost, nil
}

// Create creates a new material cost
func (r *MaterialCostRepository) Create(cost *models.MaterialCost) error {
	return r.db.Create(cost).Error
}

// Update updates an existing material cost
func (r *MaterialCostRepository) Update(cost *models.MaterialCost) error {
	return r.db.Save(cost).Error
}

// Delete soft-deletes a material cost by setting is_active to false
func (r *MaterialCostRepository) Delete(id uint) error {
	return r.db.Model(&models.MaterialCost{}).Where("id = ?", id).Update("is_active", false).Error
}
