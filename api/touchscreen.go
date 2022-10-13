package api

// Touchscreen is the interface of a touchscreen.
type Touchscreen interface {
	Tap(x float64, y float64)
}
