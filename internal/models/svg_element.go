package models

import "time"

// ElementCategory represents the operation type based on color
type ElementCategory string

const (
	CategoryCut     ElementCategory = "cut"     // Red stroke (#FF0000) - cutting path
	CategoryVector  ElementCategory = "vector"  // Blue stroke (#0000FF) - vector engraving
	CategoryRaster  ElementCategory = "raster"  // Black fill (#000000) - raster engraving
	CategoryIgnored ElementCategory = "ignored" // Other colors - ignored
)

// ElementType represents the SVG element type
type ElementType string

const (
	TypePath     ElementType = "path"
	TypeRect     ElementType = "rect"
	TypeCircle   ElementType = "circle"
	TypeEllipse  ElementType = "ellipse"
	TypeLine     ElementType = "line"
	TypePolyline ElementType = "polyline"
	TypePolygon  ElementType = "polygon"
	TypeText     ElementType = "text"
)

// SVGElement represents a single element from the SVG analysis
type SVGElement struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AnalysisID uint      `gorm:"not null;index" json:"analysis_id"`
	CreatedAt  time.Time `json:"created_at"`

	// Element identification
	ElementType ElementType `gorm:"type:varchar(20);not null" json:"element_type"` // path, rect, circle, etc.
	ElementID   *string     `gorm:"type:varchar(100)" json:"element_id,omitempty"` // Original SVG id attribute

	// Color classification
	StrokeColor *string         `gorm:"type:varchar(20)" json:"stroke_color,omitempty"` // Hex color
	FillColor   *string         `gorm:"type:varchar(20)" json:"fill_color,omitempty"`   // Hex color
	Category    ElementCategory `gorm:"type:varchar(20);not null" json:"category"`      // cut, vector, raster, ignored

	// Geometry calculations (in mm)
	Length    float64 `json:"length"`     // Path/perimeter length (for cut/vector)
	Area      float64 `json:"area"`       // Filled area (for raster)
	Perimeter float64 `json:"perimeter"`  // Outer perimeter
	PointsRaw *string `gorm:"type:text" json:"-"` // Original path data (d attribute) - not in API

	// Bounding box (mm)
	BoundsMinX float64 `json:"bounds_min_x"`
	BoundsMinY float64 `json:"bounds_min_y"`
	BoundsMaxX float64 `json:"bounds_max_x"`
	BoundsMaxY float64 `json:"bounds_max_y"`

	// Relation
	Analysis *SVGAnalysis `gorm:"foreignKey:AnalysisID" json:"-"`
}

func (SVGElement) TableName() string {
	return "svg_elements"
}

// BoundingBoxWidth returns the width of the element's bounding box
func (e *SVGElement) BoundingBoxWidth() float64 {
	return e.BoundsMaxX - e.BoundsMinX
}

// BoundingBoxHeight returns the height of the element's bounding box
func (e *SVGElement) BoundingBoxHeight() float64 {
	return e.BoundsMaxY - e.BoundsMinY
}

// IsCut returns true if this element is a cut operation
func (e *SVGElement) IsCut() bool {
	return e.Category == CategoryCut
}

// IsVector returns true if this element is a vector engrave operation
func (e *SVGElement) IsVector() bool {
	return e.Category == CategoryVector
}

// IsRaster returns true if this element is a raster engrave operation
func (e *SVGElement) IsRaster() bool {
	return e.Category == CategoryRaster
}

// IsIgnored returns true if this element was ignored
func (e *SVGElement) IsIgnored() bool {
	return e.Category == CategoryIgnored
}

// ToJSON returns element data for API response
func (e *SVGElement) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"id":           e.ID,
		"element_type": e.ElementType,
		"category":     e.Category,
		"length":       e.Length,
		"area":         e.Area,
		"bounds": map[string]float64{
			"min_x": e.BoundsMinX,
			"min_y": e.BoundsMinY,
			"max_x": e.BoundsMaxX,
			"max_y": e.BoundsMaxY,
		},
	}

	if e.ElementID != nil {
		result["element_id"] = *e.ElementID
	}
	if e.StrokeColor != nil {
		result["stroke_color"] = *e.StrokeColor
	}
	if e.FillColor != nil {
		result["fill_color"] = *e.FillColor
	}

	return result
}
