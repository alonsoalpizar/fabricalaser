package repository

import (
	"time"

	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

// QuoteRepository handles quote database operations
type QuoteRepository struct {
	db *gorm.DB
}

// NewQuoteRepository creates a new repository
func NewQuoteRepository() *QuoteRepository {
	return &QuoteRepository{
		db: database.Get(),
	}
}

// Create saves a new quote
func (r *QuoteRepository) Create(quote *models.Quote) error {
	return r.db.Create(quote).Error
}

// FindByID retrieves a quote by ID
func (r *QuoteRepository) FindByID(id uint) (*models.Quote, error) {
	var quote models.Quote
	err := r.db.First(&quote, id).Error
	if err != nil {
		return nil, err
	}
	return &quote, nil
}

// FindByIDWithRelations retrieves a quote with related entities
func (r *QuoteRepository) FindByIDWithRelations(id uint) (*models.Quote, error) {
	var quote models.Quote
	err := r.db.
		Preload("Technology").
		Preload("Material").
		Preload("EngraveType").
		Preload("SVGAnalysis").
		First(&quote, id).Error
	if err != nil {
		return nil, err
	}
	return &quote, nil
}

// FindByUserID retrieves quotes for a user
func (r *QuoteRepository) FindByUserID(userID uint, limit, offset int) ([]models.Quote, error) {
	var quotes []models.Quote
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&quotes).Error
	return quotes, err
}

// FindByUserIDWithRelations retrieves quotes with related entities
func (r *QuoteRepository) FindByUserIDWithRelations(userID uint, limit, offset int) ([]models.Quote, error) {
	var quotes []models.Quote
	query := r.db.
		Preload("Technology").
		Preload("Material").
		Preload("EngraveType").
		Where("user_id = ?", userID).
		Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&quotes).Error
	return quotes, err
}

// CountByUserThisMonth counts quotes created by user in current month
func (r *QuoteRepository) CountByUserThisMonth(userID uint) (int64, error) {
	var count int64
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	err := r.db.Model(&models.Quote{}).
		Where("user_id = ? AND created_at >= ?", userID, startOfMonth).
		Count(&count).Error
	return count, err
}

// FindPendingReview retrieves quotes needing admin review
func (r *QuoteRepository) FindPendingReview(limit, offset int) ([]models.Quote, error) {
	var quotes []models.Quote
	query := r.db.
		Preload("User").
		Preload("Technology").
		Preload("Material").
		Where("status = ?", models.QuoteStatusNeedsReview).
		Order("created_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&quotes).Error
	return quotes, err
}

// UpdateStatus updates quote status with optional review info
func (r *QuoteRepository) UpdateStatus(id uint, status models.QuoteStatus, reviewedBy *uint, reviewNotes *string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if reviewedBy != nil {
		updates["reviewed_by"] = *reviewedBy
		updates["reviewed_at"] = time.Now()
	}
	if reviewNotes != nil {
		updates["review_notes"] = *reviewNotes
	}
	return r.db.Model(&models.Quote{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateFinalPrice updates the final price (admin adjustment)
func (r *QuoteRepository) UpdateFinalPrice(id uint, priceFinal float64, adjustments interface{}) error {
	updates := map[string]interface{}{
		"price_final": priceFinal,
		"updated_at":  time.Now(),
	}
	if adjustments != nil {
		updates["adjustments"] = adjustments
	}
	return r.db.Model(&models.Quote{}).Where("id = ?", id).Updates(updates).Error
}

// MarkAsConverted marks a quote as converted to an order
func (r *QuoteRepository) MarkAsConverted(id uint, orderID uint) error {
	return r.db.Model(&models.Quote{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          models.QuoteStatusConverted,
		"converted_to_id": orderID,
		"updated_at":      time.Now(),
	}).Error
}

// ExpireOldQuotes marks expired quotes
func (r *QuoteRepository) ExpireOldQuotes() (int64, error) {
	result := r.db.Model(&models.Quote{}).
		Where("status IN (?, ?, ?) AND valid_until < ?",
			models.QuoteStatusDraft, models.QuoteStatusAutoApproved, models.QuoteStatusApproved,
			time.Now()).
		Update("status", models.QuoteStatusExpired)
	return result.RowsAffected, result.Error
}

// Delete removes a quote
func (r *QuoteRepository) Delete(id uint) error {
	return r.db.Delete(&models.Quote{}, id).Error
}

// CountByUser counts total quotes for a user
func (r *QuoteRepository) CountByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Quote{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
