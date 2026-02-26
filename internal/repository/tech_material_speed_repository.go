package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrTechMaterialSpeedNotFound = errors.New("configuración de velocidad no encontrada")

type TechMaterialSpeedRepository struct {
	db *gorm.DB
}

func NewTechMaterialSpeedRepository() *TechMaterialSpeedRepository {
	return &TechMaterialSpeedRepository{
		db: database.Get(),
	}
}

// FindAll returns all active tech material speeds with technology and material info
func (r *TechMaterialSpeedRepository) FindAll() ([]models.TechMaterialSpeed, error) {
	var speeds []models.TechMaterialSpeed
	if err := r.db.Preload("Technology").Preload("Material").Where("is_active = ?", true).Find(&speeds).Error; err != nil {
		return nil, err
	}
	return speeds, nil
}

// FindByID finds a tech material speed by ID
func (r *TechMaterialSpeedRepository) FindByID(id uint) (*models.TechMaterialSpeed, error) {
	var speed models.TechMaterialSpeed
	if err := r.db.Preload("Technology").Preload("Material").First(&speed, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechMaterialSpeedNotFound
		}
		return nil, err
	}
	return &speed, nil
}

// FindByTechAndMaterial finds all speeds for a specific technology and material combination
func (r *TechMaterialSpeedRepository) FindByTechAndMaterial(techID, materialID uint) ([]models.TechMaterialSpeed, error) {
	var speeds []models.TechMaterialSpeed
	query := r.db.Preload("Technology").Preload("Material").Where("is_active = ?", true)

	if techID > 0 {
		query = query.Where("technology_id = ?", techID)
	}
	if materialID > 0 {
		query = query.Where("material_id = ?", materialID)
	}

	if err := query.Find(&speeds).Error; err != nil {
		return nil, err
	}
	return speeds, nil
}

// FindByTechMaterialThickness finds a specific speed configuration
func (r *TechMaterialSpeedRepository) FindByTechMaterialThickness(techID, materialID uint, thickness float64) (*models.TechMaterialSpeed, error) {
	var speed models.TechMaterialSpeed
	if err := r.db.Preload("Technology").Preload("Material").
		Where("technology_id = ? AND material_id = ? AND thickness = ? AND is_active = ?", techID, materialID, thickness, true).
		First(&speed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTechMaterialSpeedNotFound
		}
		return nil, err
	}
	return &speed, nil
}

// FindCompatibleTechnologies finds all compatible technologies for a material and optional thickness
func (r *TechMaterialSpeedRepository) FindCompatibleTechnologies(materialID uint, thickness float64) ([]models.TechMaterialSpeed, error) {
	var speeds []models.TechMaterialSpeed
	query := r.db.Preload("Technology").Preload("Material").
		Where("material_id = ? AND is_compatible = ? AND is_active = ?", materialID, true, true)

	// If thickness is provided (not 0), filter by it
	if thickness > 0 {
		query = query.Where("thickness = ?", thickness)
	}

	if err := query.Find(&speeds).Error; err != nil {
		return nil, err
	}
	return speeds, nil
}

// Create creates a new tech material speed
func (r *TechMaterialSpeedRepository) Create(speed *models.TechMaterialSpeed) error {
	return r.db.Create(speed).Error
}

// Update updates an existing tech material speed
func (r *TechMaterialSpeedRepository) Update(speed *models.TechMaterialSpeed) error {
	return r.db.Save(speed).Error
}

// Delete soft-deletes a tech material speed by setting is_active to false
func (r *TechMaterialSpeedRepository) Delete(id uint) error {
	return r.db.Model(&models.TechMaterialSpeed{}).Where("id = ?", id).Update("is_active", false).Error
}

// BulkCreate creates multiple tech material speeds in a transaction
func (r *TechMaterialSpeedRepository) BulkCreate(speeds []models.TechMaterialSpeed) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range speeds {
			if err := tx.Create(&speeds[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
