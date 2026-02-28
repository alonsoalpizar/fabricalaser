package svgengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// Analyzer orchestrates SVG parsing, classification, and geometry calculation
type Analyzer struct {
	parser     *Parser
	classifier *Classifier
}

// AnalysisResult contains the complete analysis of an SVG file
type AnalysisResult struct {
	// Document info
	Width  float64 // mm
	Height float64 // mm

	// Aggregated geometry by category
	CutLengthMM    float64 // Red stroke - cut paths
	VectorLengthMM float64 // Blue stroke - vector engrave
	RasterAreaMM2  float64 // Black fill - raster engrave

	// Element counts
	ElementCount int
	CutCount     int
	VectorCount  int
	RasterCount  int
	IgnoredCount int

	// Bounding box (all elements combined)
	BoundsMinX float64
	BoundsMinY float64
	BoundsMaxX float64
	BoundsMaxY float64

	// Individual elements with geometry
	Elements []ElementResult

	// Warnings and status
	Warnings []string
	Status   string // analyzed, error
	Error    string
}

// ElementResult contains analysis for a single element
type ElementResult struct {
	Type        models.ElementType
	ElementID   *string
	Category    models.ElementCategory
	StrokeColor *string
	FillColor   *string
	Length      float64
	Area        float64
	Perimeter   float64
	BoundsMinX  float64
	BoundsMinY  float64
	BoundsMaxX  float64
	BoundsMaxY  float64
	// Multi-operation flags
	HasCut    bool
	HasVector bool
	HasRaster bool
}

// NewAnalyzer creates an analyzer with default components
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		parser:     NewParser(),
		classifier: NewClassifier(),
	}
}

// Analyze performs complete analysis of SVG content
func (a *Analyzer) Analyze(svgContent string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Elements: make([]ElementResult, 0),
		Warnings: make([]string, 0),
		Status:   "analyzed",
	}

	// Step 1: Parse SVG structure
	parsed, err := a.parser.Parse(svgContent)
	if err != nil {
		result.Status = "error"
		result.Error = err.Error()
		return result, err
	}

	result.Width = parsed.Width
	result.Height = parsed.Height
	result.Warnings = append(result.Warnings, parsed.Warnings...)

	// Step 2: Classify elements by color
	classified := a.classifier.ClassifyAll(parsed.Elements)

	// Step 3: Calculate geometry with scale factor
	scaleX, scaleY := a.parser.GetScaleFactor(parsed)
	geomCalc := NewGeometryCalculator(scaleX, scaleY)

	// Initialize bounds to first valid element
	boundsInit := false

	// Step 4: Process each element
	for _, elem := range classified {
		geom := geomCalc.Calculate(elem.Raw)

		// Convert element type
		elemType := models.ElementType(elem.Raw.Type)

		// Create element result
		elemResult := ElementResult{
			Type:        elemType,
			Category:    elem.Category,
			StrokeColor: elem.StrokeColor,
			FillColor:   elem.FillColor,
			Length:      geom.Length,
			Area:        geom.Area,
			Perimeter:   geom.Perimeter,
			BoundsMinX:  geom.Bounds.MinX,
			BoundsMinY:  geom.Bounds.MinY,
			BoundsMaxX:  geom.Bounds.MaxX,
			BoundsMaxY:  geom.Bounds.MaxY,
			HasCut:      elem.HasCut,
			HasVector:   elem.HasVector,
			HasRaster:   elem.HasRaster,
		}

		if elem.Raw.ID != "" {
			elemResult.ElementID = &elem.Raw.ID
		}

		result.Elements = append(result.Elements, elemResult)
		result.ElementCount++

		// Aggregate by detected operations (an element can have multiple!)
		hasAnyOperation := false

		if elem.HasCut {
			result.CutLengthMM += geom.Length
			result.CutCount++
			hasAnyOperation = true
		}
		if elem.HasVector {
			result.VectorLengthMM += geom.Length
			result.VectorCount++
			hasAnyOperation = true
		}
		if elem.HasRaster {
			result.RasterAreaMM2 += geom.Area
			result.RasterCount++
			hasAnyOperation = true
		}
		if !hasAnyOperation {
			result.IgnoredCount++
		}

		// Update global bounds
		if geom.Bounds.MaxX > geom.Bounds.MinX || geom.Bounds.MaxY > geom.Bounds.MinY {
			if !boundsInit {
				result.BoundsMinX = geom.Bounds.MinX
				result.BoundsMinY = geom.Bounds.MinY
				result.BoundsMaxX = geom.Bounds.MaxX
				result.BoundsMaxY = geom.Bounds.MaxY
				boundsInit = true
			} else {
				result.BoundsMinX = math.Min(result.BoundsMinX, geom.Bounds.MinX)
				result.BoundsMinY = math.Min(result.BoundsMinY, geom.Bounds.MinY)
				result.BoundsMaxX = math.Max(result.BoundsMaxX, geom.Bounds.MaxX)
				result.BoundsMaxY = math.Max(result.BoundsMaxY, geom.Bounds.MaxY)
			}
		}
	}

	// Add warning if no usable elements found
	if result.CutCount == 0 && result.VectorCount == 0 && result.RasterCount == 0 {
		result.Warnings = append(result.Warnings, "No elements with standard colors found (red stroke=cut, blue stroke=vector, black fill=raster)")
	}

	return result, nil
}

// ToModel converts AnalysisResult to database model
func (a *Analyzer) ToModel(result *AnalysisResult, userID uint, filename string, svgContent string) *models.SVGAnalysis {
	// Generate file hash
	hash := sha256.Sum256([]byte(svgContent))
	fileHash := hex.EncodeToString(hash[:])

	// Convert warnings to JSON
	warningsJSON, _ := json.Marshal(result.Warnings)

	model := &models.SVGAnalysis{
		UserID:   userID,
		Filename: filename,
		FileHash: fileHash,
		FileSize: int64(len(svgContent)),
		SVGData:  svgContent,

		Width:  result.Width,
		Height: result.Height,

		CutLengthMM:    result.CutLengthMM,
		VectorLengthMM: result.VectorLengthMM,
		RasterAreaMM2:  result.RasterAreaMM2,

		ElementCount: result.ElementCount,
		CutCount:     result.CutCount,
		VectorCount:  result.VectorCount,
		RasterCount:  result.RasterCount,
		IgnoredCount: result.IgnoredCount,

		BoundsMinX: result.BoundsMinX,
		BoundsMinY: result.BoundsMinY,
		BoundsMaxX: result.BoundsMaxX,
		BoundsMaxY: result.BoundsMaxY,

		Status:   result.Status,
		Warnings: warningsJSON,
	}

	if result.Error != "" {
		model.Error = &result.Error
	}

	// Convert elements
	model.Elements = make([]models.SVGElement, 0, len(result.Elements))
	for _, elem := range result.Elements {
		modelElem := models.SVGElement{
			ElementType: elem.Type,
			ElementID:   elem.ElementID,
			StrokeColor: elem.StrokeColor,
			FillColor:   elem.FillColor,
			Category:    elem.Category,
			Length:      elem.Length,
			Area:        elem.Area,
			Perimeter:   elem.Perimeter,
			BoundsMinX:  elem.BoundsMinX,
			BoundsMinY:  elem.BoundsMinY,
			BoundsMaxX:  elem.BoundsMaxX,
			BoundsMaxY:  elem.BoundsMaxY,
		}
		model.Elements = append(model.Elements, modelElem)
	}

	return model
}

// CalculateFileHash computes SHA256 hash of content
func CalculateFileHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
