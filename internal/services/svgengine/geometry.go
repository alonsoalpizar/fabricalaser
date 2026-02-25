package svgengine

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Point represents a 2D point
type Point struct {
	X, Y float64
}

// BoundingBox represents a rectangular bounding box
type BoundingBox struct {
	MinX, MinY float64
	MaxX, MaxY float64
}

// GeometryResult contains calculated geometry for an element
type GeometryResult struct {
	Length    float64     // Path/perimeter length
	Area      float64     // Filled area
	Perimeter float64     // Outer perimeter
	Bounds    BoundingBox // Bounding box
	Points    []Point     // Linearized points (for complex paths)
}

// GeometryCalculator calculates geometry for SVG elements
type GeometryCalculator struct {
	scaleX float64 // ViewBox to mm scale
	scaleY float64
	precision float64 // Bezier subdivision precision (mm)
}

// NewGeometryCalculator creates a calculator with given scale factors
func NewGeometryCalculator(scaleX, scaleY float64) *GeometryCalculator {
	return &GeometryCalculator{
		scaleX:    scaleX,
		scaleY:    scaleY,
		precision: 0.1, // 0.1mm precision for Bezier
	}
}

// Calculate computes geometry for an element based on its type
func (g *GeometryCalculator) Calculate(elem RawElement) GeometryResult {
	switch elem.Type {
	case "rect":
		return g.calculateRect(elem.Attributes)
	case "circle":
		return g.calculateCircle(elem.Attributes)
	case "ellipse":
		return g.calculateEllipse(elem.Attributes)
	case "line":
		return g.calculateLine(elem.Attributes)
	case "polyline":
		return g.calculatePolyline(elem.Attributes, false)
	case "polygon":
		return g.calculatePolyline(elem.Attributes, true)
	case "path":
		return g.calculatePath(elem.Attributes)
	default:
		return GeometryResult{}
	}
}

// calculateRect computes geometry for a rectangle
func (g *GeometryCalculator) calculateRect(attrs map[string]string) GeometryResult {
	x := g.parseFloat(attrs["x"]) * g.scaleX
	y := g.parseFloat(attrs["y"]) * g.scaleY
	w := g.parseFloat(attrs["width"]) * g.scaleX
	h := g.parseFloat(attrs["height"]) * g.scaleY

	// Handle rounded corners (rx, ry)
	rx := g.parseFloat(attrs["rx"]) * g.scaleX
	ry := g.parseFloat(attrs["ry"]) * g.scaleY

	perimeter := 2 * (w + h)
	area := w * h

	// Adjust for rounded corners (approximate)
	if rx > 0 || ry > 0 {
		if rx == 0 {
			rx = ry
		}
		if ry == 0 {
			ry = rx
		}
		// Corner area reduction
		cornerArea := 4 * (rx * ry - math.Pi*rx*ry/4)
		area -= cornerArea
		// Perimeter: subtract corners, add arc
		perimeter = 2*(w-2*rx) + 2*(h-2*ry) + 2*math.Pi*((rx+ry)/2)
	}

	return GeometryResult{
		Length:    perimeter,
		Area:      area,
		Perimeter: perimeter,
		Bounds: BoundingBox{
			MinX: x, MinY: y,
			MaxX: x + w, MaxY: y + h,
		},
	}
}

// calculateCircle computes geometry for a circle
func (g *GeometryCalculator) calculateCircle(attrs map[string]string) GeometryResult {
	cx := g.parseFloat(attrs["cx"]) * g.scaleX
	cy := g.parseFloat(attrs["cy"]) * g.scaleY
	r := g.parseFloat(attrs["r"]) * ((g.scaleX + g.scaleY) / 2) // Average scale for radius

	perimeter := 2 * math.Pi * r
	area := math.Pi * r * r

	return GeometryResult{
		Length:    perimeter,
		Area:      area,
		Perimeter: perimeter,
		Bounds: BoundingBox{
			MinX: cx - r, MinY: cy - r,
			MaxX: cx + r, MaxY: cy + r,
		},
	}
}

// calculateEllipse computes geometry for an ellipse
func (g *GeometryCalculator) calculateEllipse(attrs map[string]string) GeometryResult {
	cx := g.parseFloat(attrs["cx"]) * g.scaleX
	cy := g.parseFloat(attrs["cy"]) * g.scaleY
	rx := g.parseFloat(attrs["rx"]) * g.scaleX
	ry := g.parseFloat(attrs["ry"]) * g.scaleY

	area := math.Pi * rx * ry
	// Ramanujan approximation for ellipse perimeter
	h := math.Pow((rx-ry)/(rx+ry), 2)
	perimeter := math.Pi * (rx + ry) * (1 + 3*h/(10+math.Sqrt(4-3*h)))

	return GeometryResult{
		Length:    perimeter,
		Area:      area,
		Perimeter: perimeter,
		Bounds: BoundingBox{
			MinX: cx - rx, MinY: cy - ry,
			MaxX: cx + rx, MaxY: cy + ry,
		},
	}
}

// calculateLine computes geometry for a line
func (g *GeometryCalculator) calculateLine(attrs map[string]string) GeometryResult {
	x1 := g.parseFloat(attrs["x1"]) * g.scaleX
	y1 := g.parseFloat(attrs["y1"]) * g.scaleY
	x2 := g.parseFloat(attrs["x2"]) * g.scaleX
	y2 := g.parseFloat(attrs["y2"]) * g.scaleY

	length := math.Sqrt(math.Pow(x2-x1, 2) + math.Pow(y2-y1, 2))

	return GeometryResult{
		Length:    length,
		Area:      0,
		Perimeter: length,
		Bounds: BoundingBox{
			MinX: math.Min(x1, x2), MinY: math.Min(y1, y2),
			MaxX: math.Max(x1, x2), MaxY: math.Max(y1, y2),
		},
		Points: []Point{{x1, y1}, {x2, y2}},
	}
}

// calculatePolyline computes geometry for polyline/polygon
func (g *GeometryCalculator) calculatePolyline(attrs map[string]string, closed bool) GeometryResult {
	pointsStr := attrs["points"]
	if pointsStr == "" {
		return GeometryResult{}
	}

	points := g.parsePointList(pointsStr)
	if len(points) < 2 {
		return GeometryResult{}
	}

	// Calculate perimeter
	var length float64
	bounds := BoundingBox{
		MinX: points[0].X, MinY: points[0].Y,
		MaxX: points[0].X, MaxY: points[0].Y,
	}

	for i := 1; i < len(points); i++ {
		length += g.distance(points[i-1], points[i])
		bounds = g.expandBounds(bounds, points[i])
	}

	// Close polygon if needed
	if closed && len(points) > 2 {
		length += g.distance(points[len(points)-1], points[0])
	}

	// Calculate area using Shoelace formula (only for closed polygons)
	var area float64
	if closed {
		area = g.shoelaceArea(points)
	}

	return GeometryResult{
		Length:    length,
		Area:      area,
		Perimeter: length,
		Bounds:    bounds,
		Points:    points,
	}
}

// calculatePath computes geometry for a path element (SVG path data)
func (g *GeometryCalculator) calculatePath(attrs map[string]string) GeometryResult {
	d := attrs["d"]
	if d == "" {
		return GeometryResult{}
	}

	points := g.pathToPoints(d)
	if len(points) < 2 {
		return GeometryResult{}
	}

	// Calculate total length and bounds
	var length float64
	bounds := BoundingBox{
		MinX: points[0].X, MinY: points[0].Y,
		MaxX: points[0].X, MaxY: points[0].Y,
	}

	for i := 1; i < len(points); i++ {
		length += g.distance(points[i-1], points[i])
		bounds = g.expandBounds(bounds, points[i])
	}

	// Calculate area using Shoelace (assumes closed path)
	area := g.shoelaceArea(points)

	return GeometryResult{
		Length:    length,
		Area:      math.Abs(area),
		Perimeter: length,
		Bounds:    bounds,
		Points:    points,
	}
}

// pathToPoints converts SVG path data to a list of points
// Supports: M, L, H, V, C, S, Q, T, A, Z (uppercase = absolute, lowercase = relative)
func (g *GeometryCalculator) pathToPoints(d string) []Point {
	points := make([]Point, 0)
	current := Point{0, 0}
	start := Point{0, 0}
	lastControl := Point{0, 0}
	lastCmd := byte(0)

	// Parse path commands
	cmdRe := regexp.MustCompile(`([MmLlHhVvCcSsQqTtAaZz])([^MmLlHhVvCcSsQqTtAaZz]*)`)
	matches := cmdRe.FindAllStringSubmatch(d, -1)

	for _, match := range matches {
		cmd := match[1][0]
		argsStr := strings.TrimSpace(match[2])
		args := g.parseNumbers(argsStr)
		isRelative := cmd >= 'a' && cmd <= 'z'
		cmdUpper := cmd
		if isRelative {
			cmdUpper = cmd - 32
		}

		switch cmdUpper {
		case 'M': // MoveTo
			for i := 0; i < len(args)-1; i += 2 {
				x, y := args[i]*g.scaleX, args[i+1]*g.scaleY
				if isRelative && i > 0 {
					x += current.X
					y += current.Y
				} else if isRelative && i == 0 && len(points) > 0 {
					x += current.X
					y += current.Y
				}
				current = Point{x, y}
				if i == 0 {
					start = current
				}
				points = append(points, current)
			}

		case 'L': // LineTo
			for i := 0; i < len(args)-1; i += 2 {
				x, y := args[i]*g.scaleX, args[i+1]*g.scaleY
				if isRelative {
					x += current.X
					y += current.Y
				}
				current = Point{x, y}
				points = append(points, current)
			}

		case 'H': // Horizontal line
			for _, x := range args {
				newX := x * g.scaleX
				if isRelative {
					newX += current.X
				}
				current = Point{newX, current.Y}
				points = append(points, current)
			}

		case 'V': // Vertical line
			for _, y := range args {
				newY := y * g.scaleY
				if isRelative {
					newY += current.Y
				}
				current = Point{current.X, newY}
				points = append(points, current)
			}

		case 'C': // Cubic Bezier
			for i := 0; i+5 < len(args); i += 6 {
				cp1 := Point{args[i] * g.scaleX, args[i+1] * g.scaleY}
				cp2 := Point{args[i+2] * g.scaleX, args[i+3] * g.scaleY}
				end := Point{args[i+4] * g.scaleX, args[i+5] * g.scaleY}
				if isRelative {
					cp1.X += current.X
					cp1.Y += current.Y
					cp2.X += current.X
					cp2.Y += current.Y
					end.X += current.X
					end.Y += current.Y
				}
				bezierPoints := g.cubicBezier(current, cp1, cp2, end)
				points = append(points, bezierPoints[1:]...) // Skip first (current)
				current = end
				lastControl = cp2
			}

		case 'S': // Smooth cubic Bezier
			for i := 0; i+3 < len(args); i += 4 {
				// Reflect last control point
				cp1 := current
				if lastCmd == 'C' || lastCmd == 'S' || lastCmd == 'c' || lastCmd == 's' {
					cp1 = Point{2*current.X - lastControl.X, 2*current.Y - lastControl.Y}
				}
				cp2 := Point{args[i] * g.scaleX, args[i+1] * g.scaleY}
				end := Point{args[i+2] * g.scaleX, args[i+3] * g.scaleY}
				if isRelative {
					cp2.X += current.X
					cp2.Y += current.Y
					end.X += current.X
					end.Y += current.Y
				}
				bezierPoints := g.cubicBezier(current, cp1, cp2, end)
				points = append(points, bezierPoints[1:]...)
				current = end
				lastControl = cp2
			}

		case 'Q': // Quadratic Bezier
			for i := 0; i+3 < len(args); i += 4 {
				cp := Point{args[i] * g.scaleX, args[i+1] * g.scaleY}
				end := Point{args[i+2] * g.scaleX, args[i+3] * g.scaleY}
				if isRelative {
					cp.X += current.X
					cp.Y += current.Y
					end.X += current.X
					end.Y += current.Y
				}
				bezierPoints := g.quadraticBezier(current, cp, end)
				points = append(points, bezierPoints[1:]...)
				current = end
				lastControl = cp
			}

		case 'T': // Smooth quadratic Bezier
			for i := 0; i+1 < len(args); i += 2 {
				cp := current
				if lastCmd == 'Q' || lastCmd == 'T' || lastCmd == 'q' || lastCmd == 't' {
					cp = Point{2*current.X - lastControl.X, 2*current.Y - lastControl.Y}
				}
				end := Point{args[i] * g.scaleX, args[i+1] * g.scaleY}
				if isRelative {
					end.X += current.X
					end.Y += current.Y
				}
				bezierPoints := g.quadraticBezier(current, cp, end)
				points = append(points, bezierPoints[1:]...)
				current = end
				lastControl = cp
			}

		case 'A': // Arc - simplified to line for now (TODO: proper arc handling)
			for i := 0; i+6 < len(args); i += 7 {
				// rx, ry, rotation, large-arc, sweep, x, y
				end := Point{args[i+5] * g.scaleX, args[i+6] * g.scaleY}
				if isRelative {
					end.X += current.X
					end.Y += current.Y
				}
				// Approximate arc with line (TODO: improve)
				points = append(points, end)
				current = end
			}

		case 'Z': // ClosePath
			if len(points) > 0 && g.distance(current, start) > 0.01 {
				points = append(points, start)
			}
			current = start
		}

		lastCmd = cmd
	}

	return points
}

// cubicBezier approximates a cubic Bezier curve with line segments
func (g *GeometryCalculator) cubicBezier(p0, p1, p2, p3 Point) []Point {
	return g.subdivideCubic(p0, p1, p2, p3, g.precision)
}

// subdivideCubic recursively subdivides a cubic Bezier curve
func (g *GeometryCalculator) subdivideCubic(p0, p1, p2, p3 Point, tolerance float64) []Point {
	// Check if curve is flat enough
	d1 := g.pointToLineDistance(p1, p0, p3)
	d2 := g.pointToLineDistance(p2, p0, p3)

	if d1 < tolerance && d2 < tolerance {
		return []Point{p0, p3}
	}

	// De Casteljau subdivision
	p01 := g.midpoint(p0, p1)
	p12 := g.midpoint(p1, p2)
	p23 := g.midpoint(p2, p3)
	p012 := g.midpoint(p01, p12)
	p123 := g.midpoint(p12, p23)
	p0123 := g.midpoint(p012, p123)

	left := g.subdivideCubic(p0, p01, p012, p0123, tolerance)
	right := g.subdivideCubic(p0123, p123, p23, p3, tolerance)

	return append(left, right[1:]...)
}

// quadraticBezier approximates a quadratic Bezier curve with line segments
func (g *GeometryCalculator) quadraticBezier(p0, p1, p2 Point) []Point {
	// Convert to cubic
	cp1 := Point{p0.X + 2.0/3.0*(p1.X-p0.X), p0.Y + 2.0/3.0*(p1.Y-p0.Y)}
	cp2 := Point{p2.X + 2.0/3.0*(p1.X-p2.X), p2.Y + 2.0/3.0*(p1.Y-p2.Y)}
	return g.cubicBezier(p0, cp1, cp2, p2)
}

// shoelaceArea calculates polygon area using Shoelace formula
func (g *GeometryCalculator) shoelaceArea(points []Point) float64 {
	if len(points) < 3 {
		return 0
	}

	var area float64
	n := len(points)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += points[i].X * points[j].Y
		area -= points[j].X * points[i].Y
	}

	return math.Abs(area) / 2
}

// Helper functions

func (g *GeometryCalculator) parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func (g *GeometryCalculator) parseNumbers(s string) []float64 {
	nums := make([]float64, 0)
	re := regexp.MustCompile(`-?[\d.]+(?:[eE][+-]?\d+)?`)
	matches := re.FindAllString(s, -1)
	for _, m := range matches {
		if v, err := strconv.ParseFloat(m, 64); err == nil {
			nums = append(nums, v)
		}
	}
	return nums
}

func (g *GeometryCalculator) parsePointList(s string) []Point {
	nums := g.parseNumbers(s)
	points := make([]Point, 0, len(nums)/2)
	for i := 0; i < len(nums)-1; i += 2 {
		points = append(points, Point{
			X: nums[i] * g.scaleX,
			Y: nums[i+1] * g.scaleY,
		})
	}
	return points
}

func (g *GeometryCalculator) distance(p1, p2 Point) float64 {
	return math.Sqrt(math.Pow(p2.X-p1.X, 2) + math.Pow(p2.Y-p1.Y, 2))
}

func (g *GeometryCalculator) midpoint(p1, p2 Point) Point {
	return Point{(p1.X + p2.X) / 2, (p1.Y + p2.Y) / 2}
}

func (g *GeometryCalculator) pointToLineDistance(p, lineStart, lineEnd Point) float64 {
	dx := lineEnd.X - lineStart.X
	dy := lineEnd.Y - lineStart.Y
	if dx == 0 && dy == 0 {
		return g.distance(p, lineStart)
	}
	t := ((p.X-lineStart.X)*dx + (p.Y-lineStart.Y)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	closest := Point{lineStart.X + t*dx, lineStart.Y + t*dy}
	return g.distance(p, closest)
}

func (g *GeometryCalculator) expandBounds(b BoundingBox, p Point) BoundingBox {
	return BoundingBox{
		MinX: math.Min(b.MinX, p.X),
		MinY: math.Min(b.MinY, p.Y),
		MaxX: math.Max(b.MaxX, p.X),
		MaxY: math.Max(b.MaxY, p.Y),
	}
}
