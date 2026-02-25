package repository

import (
	"errors"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

var ErrVolumeDiscountNotFound = errors.New("descuento por volumen no encontrado")

type VolumeDiscountRepository struct {
	db *gorm.DB
}

func NewVolumeDiscountRepository() *VolumeDiscountRepository {
	return &VolumeDiscountRepository{
		db: database.Get(),
	}
}

// FindAll returns all active volume discounts ordered by min_qty
func (r *VolumeDiscountRepository) FindAll() ([]models.VolumeDiscount, error) {
	var discounts []models.VolumeDiscount
	if err := r.db.Where("is_active = ?", true).Order("min_qty").Find(&discounts).Error; err != nil {
		return nil, err
	}
	return discounts, nil
}

// FindByID finds a volume discount by ID
func (r *VolumeDiscountRepository) FindByID(id uint) (*models.VolumeDiscount, error) {
	var discount models.VolumeDiscount
	if err := r.db.First(&discount, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVolumeDiscountNotFound
		}
		return nil, err
	}
	return &discount, nil
}

// FindByQuantity finds the applicable discount for a given quantity
func (r *VolumeDiscountRepository) FindByQuantity(qty int) (*models.VolumeDiscount, error) {
	var discount models.VolumeDiscount
	query := r.db.Where("is_active = ? AND min_qty <= ?", true, qty)
	query = query.Where("max_qty >= ? OR max_qty IS NULL", qty)
	if err := query.Order("discount_pct DESC").First(&discount).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVolumeDiscountNotFound
		}
		return nil, err
	}
	return &discount, nil
}

// Create creates a new volume discount
func (r *VolumeDiscountRepository) Create(discount *models.VolumeDiscount) error {
	return r.db.Create(discount).Error
}

// Update updates an existing volume discount
func (r *VolumeDiscountRepository) Update(discount *models.VolumeDiscount) error {
	return r.db.Save(discount).Error
}

// Delete soft-deletes a volume discount by setting is_active to false
func (r *VolumeDiscountRepository) Delete(id uint) error {
	return r.db.Model(&models.VolumeDiscount{}).Where("id = ?", id).Update("is_active", false).Error
}
