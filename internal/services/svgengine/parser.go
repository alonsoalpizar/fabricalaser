package svgengine

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SVGDocument represents the parsed SVG root element
type SVGDocument struct {
	XMLName xml.Name `xml:"svg"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	ViewBox string   `xml:"viewBox,attr"`

	// Nested elements (we parse these manually due to mixed content)
	RawXML string `xml:",innerxml"`
}

// RawElement represents a generic SVG element with its attributes
type RawElement struct {
	Type       string            // path, rect, circle, etc.
	ID         string            // id attribute
	Attributes map[string]string // All attributes
}

// ParsedSVG contains the result of parsing an SVG document
type ParsedSVG struct {
	Width    float64       // Document width in mm
	Height   float64       // Document height in mm
	ViewBox  ViewBox       // ViewBox for coordinate transformation
	Elements []RawElement  // All extracted elements
	Warnings []string      // Non-fatal issues found
}

// ViewBox represents the SVG viewBox attribute
type ViewBox struct {
	MinX   float64
	MinY   float64
	Width  float64
	Height float64
	Valid  bool
}

// Parser handles SVG document parsing
type Parser struct {
	defaultUnit string  // Default unit when not specified (mm, px, etc.)
	dpi         float64 // DPI for px to mm conversion
}

// NewParser creates a new SVG parser with default settings
func NewParser() *Parser {
	return &Parser{
		defaultUnit: "mm",
		dpi:         96, // Standard screen DPI for px conversion
	}
}

// Parse extracts structure and elements from SVG content
func (p *Parser) Parse(svgContent string) (*ParsedSVG, error) {
	result := &ParsedSVG{
		Elements: make([]RawElement, 0),
		Warnings: make([]string, 0),
	}

	// Parse the SVG root element
	var doc SVGDocument
	if err := xml.Unmarshal([]byte(svgContent), &doc); err != nil {
		return nil, fmt.Errorf("invalid SVG XML: %w", err)
	}

	// Parse viewBox
	result.ViewBox = p.parseViewBox(doc.ViewBox)

	// Parse width/height
	result.Width = p.parseLength(doc.Width, result.ViewBox.Width)
	result.Height = p.parseLength(doc.Height, result.ViewBox.Height)

	// If no explicit dimensions, use viewBox
	if result.Width == 0 && result.ViewBox.Valid {
		result.Width = result.ViewBox.Width
	}
	if result.Height == 0 && result.ViewBox.Valid {
		result.Height = result.ViewBox.Height
	}

	// Default to 100x100 if nothing specified
	if result.Width == 0 {
		result.Width = 100
		result.Warnings = append(result.Warnings, "No width specified, using default 100mm")
	}
	if result.Height == 0 {
		result.Height = 100
		result.Warnings = append(result.Warnings, "No height specified, using default 100mm")
	}

	// Extract elements from inner XML
	elements := p.extractElements(doc.RawXML)
	result.Elements = elements

	if len(elements) == 0 {
		result.Warnings = append(result.Warnings, "No drawable elements found in SVG")
	}

	return result, nil
}

// parseViewBox parses the viewBox attribute "minX minY width height"
func (p *Parser) parseViewBox(viewBox string) ViewBox {
	vb := ViewBox{}
	if viewBox == "" {
		return vb
	}

	// Split by space or comma
	parts := regexp.MustCompile(`[\s,]+`).Split(strings.TrimSpace(viewBox), -1)
	if len(parts) != 4 {
		return vb
	}

	var err error
	vb.MinX, err = strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return vb
	}
	vb.MinY, err = strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return vb
	}
	vb.Width, err = strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return vb
	}
	vb.Height, err = strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return vb
	}

	vb.Valid = true
	return vb
}

// parseLength converts SVG length value to mm
func (p *Parser) parseLength(value string, fallback float64) float64 {
	if value == "" {
		return fallback
	}

	value = strings.TrimSpace(value)

	// Extract number and unit
	re := regexp.MustCompile(`^([\d.]+)(mm|cm|in|pt|px|%)?$`)
	matches := re.FindStringSubmatch(value)
	if len(matches) < 2 {
		return fallback
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return fallback
	}

	unit := "mm"
	if len(matches) >= 3 && matches[2] != "" {
		unit = matches[2]
	}

	// Convert to mm
	switch unit {
	case "mm":
		return num
	case "cm":
		return num * 10
	case "in":
		return num * 25.4
	case "pt":
		return num * 25.4 / 72
	case "px":
		return num * 25.4 / p.dpi
	case "%":
		return fallback * num / 100
	default:
		return num // Assume mm
	}
}

// extractElements parses inner XML to find drawable elements
func (p *Parser) extractElements(innerXML string) []RawElement {
	elements := make([]RawElement, 0)

	// Element types we care about
	elementTypes := []string{"path", "rect", "circle", "ellipse", "line", "polyline", "polygon"}

	for _, elemType := range elementTypes {
		// Find all elements of this type
		pattern := fmt.Sprintf(`<%s\s+([^>]*)/?>`+`|<%s\s+([^>]*)>.*?</%s>`, elemType, elemType, elemType)
		re := regexp.MustCompile(`(?is)` + pattern)
		matches := re.FindAllStringSubmatch(innerXML, -1)

		for _, match := range matches {
			// Get the attributes string (from either self-closing or regular tag)
			attrStr := match[1]
			if attrStr == "" {
				attrStr = match[2]
			}

			attrs := p.parseAttributes(attrStr)
			if len(attrs) == 0 {
				continue
			}

			elem := RawElement{
				Type:       elemType,
				ID:         attrs["id"],
				Attributes: attrs,
			}
			elements = append(elements, elem)
		}
	}

	// Also look for elements inside <g> groups (recursive extraction already happens
	// because we search the entire innerXML)

	return elements
}

// parseAttributes extracts key="value" pairs from an attribute string
func (p *Parser) parseAttributes(attrStr string) map[string]string {
	attrs := make(map[string]string)

	// Match attribute="value" or attribute='value'
	re := regexp.MustCompile(`(\w[\w-]*)\s*=\s*["']([^"']*)["']`)
	matches := re.FindAllStringSubmatch(attrStr, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			attrs[match[1]] = match[2]
		}
	}

	return attrs
}

// GetScaleFactor returns the scale factor to convert viewBox coordinates to mm
func (p *Parser) GetScaleFactor(parsed *ParsedSVG) (scaleX, scaleY float64) {
	if !parsed.ViewBox.Valid || parsed.ViewBox.Width == 0 || parsed.ViewBox.Height == 0 {
		return 1, 1
	}
	scaleX = parsed.Width / parsed.ViewBox.Width
	scaleY = parsed.Height / parsed.ViewBox.Height
	return scaleX, scaleY
}
