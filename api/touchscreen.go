package api

// TouchscreenAPI is the interface of a touchscreen.
type TouchscreenAPI interface {
	Tap(x float64, y float64)
}
