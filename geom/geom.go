// Package geom holds the integer canvas geometry shared by layout,
// navigation and rendering, so none of them has to import another just to
// talk about where a box is.
package geom

// Point is a cell on the canvas.
type Point struct {
	X, Y int
}

// Size is how many cells something covers, across and down.
type Size struct {
	Width, Height int
}

// Rect is an axis-aligned box in canvas coordinates.
type Rect struct {
	X, Y, Width, Height int
}

// Center returns the rectangle's center point, as floats so distance/angle
// math doesn't need to round on every step.
func (r Rect) Center() (float64, float64) {
	return float64(r.X) + float64(r.Width)/2, float64(r.Y) + float64(r.Height)/2
}

// Contains reports whether the cell (x, y) lies inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// Overlaps reports whether r and o share at least one cell.
func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.X+o.Width && o.X < r.X+r.Width && r.Y < o.Y+o.Height && o.Y < r.Y+r.Height
}

// Union is the smallest rectangle covering both r and o.
func (r Rect) Union(o Rect) Rect {
	x, y := min(r.X, o.X), min(r.Y, o.Y)
	return Rect{
		X:      x,
		Y:      y,
		Width:  max(r.X+r.Width, o.X+o.Width) - x,
		Height: max(r.Y+r.Height, o.Y+o.Height) - y,
	}
}
