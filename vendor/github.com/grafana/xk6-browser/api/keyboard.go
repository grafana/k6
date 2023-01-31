package api

import "github.com/dop251/goja"

// Keyboard is the interface of a keyboard input device.
type Keyboard interface {
	Down(key string)
	InsertText(char string)
	Press(key string, opts goja.Value)
	Type(text string, opts goja.Value)
	Up(key string)
}
