package cellbuf

import "errors"

// ErrOutOfBounds is returned when the given x, y position is out of bounds.
var ErrOutOfBounds = errors.New("out of bounds")
