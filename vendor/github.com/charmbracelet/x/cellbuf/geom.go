package cellbuf

import (
	"image"
)

// Position represents an x, y position.
type Position = image.Point

// Pos is a shorthand for Position{X: x, Y: y}.
func Pos(x, y int) Position {
	return image.Pt(x, y)
}

// Rectange represents a rectangle.
type Rectangle = image.Rectangle

// Rect is a shorthand for Rectangle.
func Rect(x, y, w, h int) Rectangle {
	return image.Rect(x, y, x+w, y+h)
}
