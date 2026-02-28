package svgengine

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/alonsoalpizar/fabricalaser/internal/models"
)

// Color convention constants (from roadmap)
// Red stroke = Cut, Blue stroke = Vector engrave, Black fill = Raster engrave
const (
	colorTolerance = 25 // ±10% of 255 ≈ 25
)

// Reference colors (hex)
var (
	colorRed   = RGB{255, 0, 0}     // #FF0000 - Cut
	colorBlue  = RGB{0, 0, 255}     // #0000FF - Vector
	colorBlack = RGB{0, 0, 0}       // #000000 - Raster
)

// RGB represents a color in RGB space
type RGB struct {
	R, G, B int
}

// Classifier determines element category based on color conventions
type Classifier struct {
	tolerance int
}

// ClassifiedElement contains the raw element with its classified operations
// An element can have multiple operations (e.g., blue stroke + black fill = vector + raster)
type ClassifiedElement struct {
	Raw         RawElement
	Category    models.ElementCategory // Primary category (for backward compatibility)
	StrokeColor *string
	FillColor   *string
	// Multi-operation flags
	HasCut    bool // Red stroke detected
	HasVector bool // Blue stroke detected
	HasRaster bool // Black fill detected
}

// NewClassifier creates a classifier with default tolerance
func NewClassifier() *Classifier {
	return &Classifier{
		tolerance: colorTolerance,
	}
}

// Classify determines ALL operations of an element based on its stroke/fill colors
// An element can have multiple operations (e.g., stroke azul + fill negro = vector + raster)
func (c *Classifier) Classify(elem RawElement) ClassifiedElement {
	result := ClassifiedElement{
		Raw:      elem,
		Category: models.CategoryIgnored,
	}

	// Extract stroke and fill colors
	strokeStr := c.getColorAttribute(elem.Attributes, "stroke")
	fillStr := c.getColorAttribute(elem.Attributes, "fill")

	if strokeStr != "" {
		result.StrokeColor = &strokeStr
	}
	if fillStr != "" {
		result.FillColor = &fillStr
	}

	// Check stroke color for Cut (red) or Vector (blue)
	if strokeStr != "" && strokeStr != "none" {
		strokeRGB := c.parseColor(strokeStr)
		if strokeRGB != nil {
			if c.isColorMatch(*strokeRGB, colorRed) {
				result.HasCut = true
				result.Category = models.CategoryCut // Primary category
			} else if c.isColorMatch(*strokeRGB, colorBlue) {
				result.HasVector = true
				if result.Category == models.CategoryIgnored {
					result.Category = models.CategoryVector
				}
			}
		}
	}

	// Check fill color for Raster (black)
	// This is independent of stroke - an element can have both!
	if fillStr != "" && fillStr != "none" {
		fillRGB := c.parseColor(fillStr)
		if fillRGB != nil {
			if c.isColorMatch(*fillRGB, colorBlack) {
				result.HasRaster = true
				if result.Category == models.CategoryIgnored {
					result.Category = models.CategoryRaster
				}
			}
		}
	}

	return result
}

// ClassifyAll classifies a slice of elements
func (c *Classifier) ClassifyAll(elements []RawElement) []ClassifiedElement {
	result := make([]ClassifiedElement, 0, len(elements))
	for _, elem := range elements {
		result = append(result, c.Classify(elem))
	}
	return result
}

// getColorAttribute gets color from attribute, checking style attribute too
func (c *Classifier) getColorAttribute(attrs map[string]string, name string) string {
	// Direct attribute
	if val, ok := attrs[name]; ok {
		return strings.TrimSpace(val)
	}

	// Check style attribute (CSS inline style)
	if style, ok := attrs["style"]; ok {
		// Parse style="stroke:#ff0000;fill:none"
		pattern := regexp.MustCompile(`(?i)` + name + `\s*:\s*([^;]+)`)
		if matches := pattern.FindStringSubmatch(style); len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// parseColor converts various color formats to RGB
func (c *Classifier) parseColor(colorStr string) *RGB {
	colorStr = strings.TrimSpace(strings.ToLower(colorStr))

	if colorStr == "" || colorStr == "none" || colorStr == "transparent" {
		return nil
	}

	// Named colors (common ones for laser work)
	namedColors := map[string]RGB{
		"red":     {255, 0, 0},
		"blue":    {0, 0, 255},
		"black":   {0, 0, 0},
		"white":   {255, 255, 255},
		"green":   {0, 128, 0},
		"yellow":  {255, 255, 0},
		"cyan":    {0, 255, 255},
		"magenta": {255, 0, 255},
	}

	if rgb, ok := namedColors[colorStr]; ok {
		return &rgb
	}

	// Hex format: #RGB or #RRGGBB
	if strings.HasPrefix(colorStr, "#") {
		hex := colorStr[1:]

		if len(hex) == 3 {
			// #RGB -> #RRGGBB
			hex = string(hex[0]) + string(hex[0]) +
				string(hex[1]) + string(hex[1]) +
				string(hex[2]) + string(hex[2])
		}

		if len(hex) == 6 {
			r, err1 := strconv.ParseInt(hex[0:2], 16, 64)
			g, err2 := strconv.ParseInt(hex[2:4], 16, 64)
			b, err3 := strconv.ParseInt(hex[4:6], 16, 64)
			if err1 == nil && err2 == nil && err3 == nil {
				return &RGB{int(r), int(g), int(b)}
			}
		}
	}

	// RGB format: rgb(255, 0, 0) or rgb(255,0,0)
	rgbPattern := regexp.MustCompile(`rgb\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)`)
	if matches := rgbPattern.FindStringSubmatch(colorStr); len(matches) == 4 {
		r, _ := strconv.Atoi(matches[1])
		g, _ := strconv.Atoi(matches[2])
		b, _ := strconv.Atoi(matches[3])
		return &RGB{r, g, b}
	}

	return nil
}

// isColorMatch checks if two colors match within tolerance
func (c *Classifier) isColorMatch(color, target RGB) bool {
	return abs(color.R-target.R) <= c.tolerance &&
		abs(color.G-target.G) <= c.tolerance &&
		abs(color.B-target.B) <= c.tolerance
}

// abs returns absolute value of int
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// CountByCategory counts elements by category
func CountByCategory(elements []ClassifiedElement) (cut, vector, raster, ignored int) {
	for _, e := range elements {
		switch e.Category {
		case models.CategoryCut:
			cut++
		case models.CategoryVector:
			vector++
		case models.CategoryRaster:
			raster++
		case models.CategoryIgnored:
			ignored++
		}
	}
	return
}
