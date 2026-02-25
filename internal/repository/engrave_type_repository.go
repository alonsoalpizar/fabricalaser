package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrEngraveTypeNotFound = errors.New("tipo de grabado no encontrado")

type EngraveTypeRepository struct {
	db *gorm.DB
}

func NewEngraveTypeRepository() *EngraveTypeRepository {
	return &EngraveTypeRepository{
		db: database.Get(),
	}
}

// FindAll returns all active engrave types
func (r *EngraveTypeRepository) FindAll() ([]models.EngraveType, error) {
	var types []models.EngraveType
	if err := r.db.Where("is_active = ?", true).Order("factor").Find(&types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// FindByID finds an engrave type by ID
func (r *EngraveTypeRepository) FindByID(id uint) (*models.EngraveType, error) {
	var engraveType models.EngraveType
	if err := r.db.First(&engraveType, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEngraveTypeNotFound
		}
		return nil, err
	}
	return &engraveType, nil
}

// Create creates a new engrave type
func (r *EngraveTypeRepository) Create(engraveType *models.EngraveType) error {
	return r.db.Create(engraveType).Error
}

// Update updates an existing engrave type
func (r *EngraveTypeRepository) Update(engraveType *models.EngraveType) error {
	return r.db.Save(engraveType).Error
}

// Delete soft-deletes an engrave type by setting is_active to false
func (r *EngraveTypeRepository) Delete(id uint) error {
	return r.db.Model(&models.EngraveType{}).Where("id = ?", id).Update("is_active", false).Error
}
