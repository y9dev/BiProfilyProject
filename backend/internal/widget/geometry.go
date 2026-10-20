package widget

import (
	"fmt"
	"math"
	"strings"
)

// Radar canvas: viewBox 0 0 200 200, centre (100,100), outer ring radius 72.
const (
	cx, cy, radius = 100.0, 100.0, 72.0
	minRadarAxes   = 3 // below this a polygon is degenerate; the radar is omitted
)

// Point is a position in the radar viewBox.
type Point struct{ X, Y float64 }

// clampPct bounds a score to the 0..100 scale (NaN counts as 0).
func clampPct(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	return math.Min(v, 100)
}

// AxisPoint is the point of axis i of n at the given value (0..100). Axis 0 points
// straight up; axes run clockwise at equal angles.
func AxisPoint(i, n int, value float64) Point {
	if n <= 0 {
		return Point{cx, cy}
	}
	ang := -math.Pi/2 + 2*math.Pi*float64(i)/float64(n)
	r := radius * clampPct(value) / 100
	return Point{cx + r*math.Cos(ang), cy + r*math.Sin(ang)}
}

func (p Point) String() string { return fmt.Sprintf("%.1f,%.1f", p.X, p.Y) }

// Polygon renders a "points" attribute for the given values (one per axis).
func Polygon(values []float64) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = AxisPoint(i, len(values), v).String()
	}
	return strings.Join(parts, " ")
}

// ring is a grid polygon at a constant value on all n axes.
func ring(n int, value float64) string {
	vals := make([]float64, n)
	for i := range vals {
		vals[i] = value
	}
	return Polygon(vals)
}
