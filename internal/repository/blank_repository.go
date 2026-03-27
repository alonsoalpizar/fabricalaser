package repository

import (
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

type BlankRepository struct {
	db *gorm.DB
}

func NewBlankRepository() *BlankRepository {
	return &BlankRepository{db: database.Get()}
}

// FindAll retorna todos los blanks activos, ordenados por destacados primero
// y luego por número de consultas descendente.
func (r *BlankRepository) FindAll() ([]models.Blank, error) {
	var blanks []models.Blank
	result := r.db.Where("is_active = true").
		Order("is_featured DESC, quote_count DESC, name ASC").
		Find(&blanks)
	return blanks, result.Error
}

// FindAllAdmin retorna todos los blanks (activos e inactivos) para el panel admin.
func (r *BlankRepository) FindAllAdmin() ([]models.Blank, error) {
	var blanks []models.Blank
	result := r.db.Order("is_active DESC, is_featured DESC, quote_count DESC, name ASC").Find(&blanks)
	return blanks, result.Error
}

// FindByCategory retorna blanks activos de una categoría específica.
func (r *BlankRepository) FindByCategory(category string) ([]models.Blank, error) {
	var blanks []models.Blank
	result := r.db.Where("is_active = true AND category = ?", category).
		Order("is_featured DESC, quote_count DESC, name ASC").
		Find(&blanks)
	return blanks, result.Error
}

// FindByID retorna un blank por ID (activo o no).
func (r *BlankRepository) FindByID(id uint) (*models.Blank, error) {
	var blank models.Blank
	result := r.db.First(&blank, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &blank, nil
}

// Create inserta un nuevo blank.
func (r *BlankRepository) Create(blank *models.Blank) error {
	return r.db.Create(blank).Error
}

// Update guarda los cambios de un blank existente.
func (r *BlankRepository) Update(blank *models.Blank) error {
	return r.db.Save(blank).Error
}

// Delete realiza soft-delete (is_active = false).
func (r *BlankRepository) Delete(id uint) error {
	return r.db.Model(&models.Blank{}).Where("id = ?", id).Update("is_active", false).Error
}

// UpdateStock actualiza el stock de un blank. operation puede ser "add" o "set".
func (r *BlankRepository) UpdateStock(id uint, qty int, operation string) error {
	if operation == "add" {
		return r.db.Model(&models.Blank{}).
			Where("id = ?", id).
			UpdateColumn("stock_qty", gorm.Expr("stock_qty + ?", qty)).Error
	}
	// "set"
	return r.db.Model(&models.Blank{}).
		Where("id = ?", id).
		UpdateColumn("stock_qty", qty).Error
}

// ToggleFeatured invierte el valor de is_featured de un blank.
func (r *BlankRepository) ToggleFeatured(id uint) error {
	return r.db.Model(&models.Blank{}).
		Where("id = ?", id).
		UpdateColumn("is_featured", gorm.Expr("NOT is_featured")).Error
}

// IncrementQuoteCount incrementa de forma atómica el contador de consultas.
func (r *BlankRepository) IncrementQuoteCount(id uint) error {
	return r.db.Model(&models.Blank{}).
		Where("id = ?", id).
		UpdateColumn("quote_count", gorm.Expr("quote_count + 1")).Error
}
