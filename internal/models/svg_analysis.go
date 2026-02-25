package models

import (
	"time"

	"gorm.io/datatypes"
)

// SVGAnalysis represents the result of analyzing an SVG file
type SVGAnalysis struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Filename  string    `gorm:"type:varchar(255);not null" json:"filename"`
	FileHash  string    `gorm:"type:varchar(64);index" json:"file_hash"` // SHA256 for dedup
	FileSize  int64     `json:"file_size"`                               // bytes
	SVGData   string    `gorm:"type:text" json:"-"`                      // Original SVG content (not in API response)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Dimensions (from viewBox or width/height)
	Width  float64 `json:"width"`  // mm
	Height float64 `json:"height"` // mm

	// Aggregated geometry from color classification
	CutLengthMM    float64 `json:"cut_length_mm"`    // Red stroke - total cut path length
	VectorLengthMM float64 `json:"vector_length_mm"` // Blue stroke - vector engrave length
	RasterAreaMM2  float64 `json:"raster_area_mm2"`  // Black fill - raster engrave area

	// Element counts
	ElementCount int `json:"element_count"` // Total elements processed
	CutCount     int `json:"cut_count"`     // Red elements
	VectorCount  int `json:"vector_count"`  // Blue elements
	RasterCount  int `json:"raster_count"`  // Black elements
	IgnoredCount int `json:"ignored_count"` // Other colors (ignored)

	// Bounding box (mm)
	BoundsMinX float64 `json:"bounds_min_x"`
	BoundsMinY float64 `json:"bounds_min_y"`
	BoundsMaxX float64 `json:"bounds_max_x"`
	BoundsMaxY float64 `json:"bounds_max_y"`

	// Status and validation
	Status   string         `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, analyzed, error
	Warnings datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"warnings"`          // Array of warning messages
	Error    *string        `gorm:"type:text" json:"error,omitempty"`                 // Error message if status=error

	// Relations
	User     *User        `gorm:"foreignKey:UserID" json:"-"`
	Elements []SVGElement `gorm:"foreignKey:AnalysisID" json:"elements,omitempty"`
}

func (SVGAnalysis) TableName() string {
	return "svg_analyses"
}

// TotalArea returns the total working area (bounding box)
func (a *SVGAnalysis) TotalArea() float64 {
	return (a.BoundsMaxX - a.BoundsMinX) * (a.BoundsMaxY - a.BoundsMinY)
}

// HasCutOperations returns true if there are cut paths
func (a *SVGAnalysis) HasCutOperations() bool {
	return a.CutLengthMM > 0
}

// HasEngraveOperations returns true if there are engrave operations (vector or raster)
func (a *SVGAnalysis) HasEngraveOperations() bool {
	return a.VectorLengthMM > 0 || a.RasterAreaMM2 > 0
}

// ComplexityFactor returns a factor indicating design complexity
// Used for auto-approval classification
func (a *SVGAnalysis) ComplexityFactor() float64 {
	area := a.TotalArea()
	if area <= 0 {
		return 0
	}
	// Factor = (cut + vector length) / sqrt(area)
	// Lower is simpler, higher is more complex
	totalLength := a.CutLengthMM + a.VectorLengthMM
	return totalLength / sqrt(area)
}

// sqrt helper (avoid importing math for one function)
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// ToSummary returns a summary suitable for API responses
func (a *SVGAnalysis) ToSummary() map[string]interface{} {
	return map[string]interface{}{
		"id":               a.ID,
		"filename":         a.Filename,
		"width_mm":         a.Width,
		"height_mm":        a.Height,
		"cut_length_mm":    a.CutLengthMM,
		"vector_length_mm": a.VectorLengthMM,
		"raster_area_mm2":  a.RasterAreaMM2,
		"element_count":    a.ElementCount,
		"status":           a.Status,
		"warnings":         a.Warnings,
		"created_at":       a.CreatedAt,
	}
}
