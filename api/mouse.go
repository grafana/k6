package api

import "github.com/dop251/goja"

// Mouse is the interface of a mouse input device.
type Mouse interface {
	Click(x float64, y float64, opts goja.Value)
	DblClick(x float64, y float64, opts goja.Value)
	Down(x float64, y float64, opts goja.Value)
	Move(x float64, y float64, opts goja.Value)
	Up(x float64, y float64, opts goja.Value)
	// Wheel(opts goja.Value)
}
