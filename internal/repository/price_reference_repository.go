package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrPriceReferenceNotFound = errors.New("referencia de precio no encontrada")

type PriceReferenceRepository struct {
	db *gorm.DB
}

func NewPriceReferenceRepository() *PriceReferenceRepository {
	return &PriceReferenceRepository{
		db: database.Get(),
	}
}

// FindAll returns all active price references
func (r *PriceReferenceRepository) FindAll() ([]models.PriceReference, error) {
	var refs []models.PriceReference
	if err := r.db.Where("is_active = ?", true).Order("service_type").Find(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

// FindByID finds a price reference by ID
func (r *PriceReferenceRepository) FindByID(id uint) (*models.PriceReference, error) {
	var ref models.PriceReference
	if err := r.db.First(&ref, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPriceReferenceNotFound
		}
		return nil, err
	}
	return &ref, nil
}

// FindByServiceType finds a price reference by service type
func (r *PriceReferenceRepository) FindByServiceType(serviceType string) (*models.PriceReference, error) {
	var ref models.PriceReference
	if err := r.db.Where("service_type = ? AND is_active = ?", serviceType, true).First(&ref).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPriceReferenceNotFound
		}
		return nil, err
	}
	return &ref, nil
}

// Create creates a new price reference
func (r *PriceReferenceRepository) Create(ref *models.PriceReference) error {
	return r.db.Create(ref).Error
}

// Update updates an existing price reference
func (r *PriceReferenceRepository) Update(ref *models.PriceReference) error {
	return r.db.Save(ref).Error
}

// Delete soft-deletes a price reference by setting is_active to false
func (r *PriceReferenceRepository) Delete(id uint) error {
	return r.db.Model(&models.PriceReference{}).Where("id = ?", id).Update("is_active", false).Error
}
