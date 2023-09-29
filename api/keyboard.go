package api

import "github.com/dop251/goja"

// KeyboardAPI is the interface of a keyboard input device.
type KeyboardAPI interface {
	Down(key string)
	InsertText(char string)
	Press(key string, opts goja.Value)
	Type(text string, opts goja.Value)
	Up(key string)
}
