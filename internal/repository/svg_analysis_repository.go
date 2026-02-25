package repository

import (
	"github.com/alonsoalpizar/fabricalaser/internal/database"
	"github.com/alonsoalpizar/fabricalaser/internal/models"
	"gorm.io/gorm"
)

// SVGAnalysisRepository handles SVG analysis database operations
type SVGAnalysisRepository struct {
	db *gorm.DB
}

// NewSVGAnalysisRepository creates a new repository
func NewSVGAnalysisRepository() *SVGAnalysisRepository {
	return &SVGAnalysisRepository{
		db: database.Get(),
	}
}

// Create saves a new SVG analysis with its elements
func (r *SVGAnalysisRepository) Create(analysis *models.SVGAnalysis) error {
	return r.db.Create(analysis).Error
}

// FindByID retrieves an analysis by ID
func (r *SVGAnalysisRepository) FindByID(id uint) (*models.SVGAnalysis, error) {
	var analysis models.SVGAnalysis
	err := r.db.First(&analysis, id).Error
	if err != nil {
		return nil, err
	}
	return &analysis, nil
}

// FindByIDWithElements retrieves an analysis with its elements
func (r *SVGAnalysisRepository) FindByIDWithElements(id uint) (*models.SVGAnalysis, error) {
	var analysis models.SVGAnalysis
	err := r.db.Preload("Elements").First(&analysis, id).Error
	if err != nil {
		return nil, err
	}
	return &analysis, nil
}

// FindByUserID retrieves analyses for a user
func (r *SVGAnalysisRepository) FindByUserID(userID uint, limit, offset int) ([]models.SVGAnalysis, error) {
	var analyses []models.SVGAnalysis
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	err := query.Find(&analyses).Error
	return analyses, err
}

// FindByFileHash finds an existing analysis by file hash (for deduplication)
func (r *SVGAnalysisRepository) FindByFileHash(userID uint, fileHash string) (*models.SVGAnalysis, error) {
	var analysis models.SVGAnalysis
	err := r.db.Where("user_id = ? AND file_hash = ?", userID, fileHash).First(&analysis).Error
	if err != nil {
		return nil, err
	}
	return &analysis, nil
}

// UpdateStatus updates the status of an analysis
func (r *SVGAnalysisRepository) UpdateStatus(id uint, status string, errorMsg *string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if errorMsg != nil {
		updates["error"] = *errorMsg
	}
	return r.db.Model(&models.SVGAnalysis{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes an analysis (cascade deletes elements)
func (r *SVGAnalysisRepository) Delete(id uint) error {
	return r.db.Delete(&models.SVGAnalysis{}, id).Error
}

// CountByUser counts total analyses for a user
func (r *SVGAnalysisRepository) CountByUser(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.SVGAnalysis{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}
