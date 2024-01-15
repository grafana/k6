// Package keyboardlayout provides keyboard key interpretation and layout validation.
package keyboardlayout

import (
	"fmt"
	"sync"
)

type KeyInput string

type KeyDefinition struct {
	Code                   string
	Key                    string
	KeyCode                int64
	KeyCodeWithoutLocation int64
	ShiftKey               string
	ShiftKeyCode           int64
	Text                   string
	Location               int64
}

type KeyboardLayout struct {
	ValidKeys map[KeyInput]bool
	Keys      map[KeyInput]KeyDefinition
}

// KeyDefinition returns true with the key definition of a given key input.
// It returns false and an empty key definition if it cannot find the key.
func (kl KeyboardLayout) KeyDefinition(key KeyInput) (KeyDefinition, bool) {
	for _, d := range kl.Keys {
		if d.Key == string(key) {
			return d, true
		}
	}
	return KeyDefinition{}, false
}

// ShiftKeyDefinition returns shift key definition of a given key input.
// It returns an empty key definition if it cannot find the key.
func (kl KeyboardLayout) ShiftKeyDefinition(key KeyInput) KeyDefinition {
	for _, d := range kl.Keys {
		if d.ShiftKey == string(key) {
			return d
		}
	}
	return KeyDefinition{}
}

//nolint:gochecknoglobals
var (
	kbdLayouts = make(map[string]KeyboardLayout)
	mx         sync.RWMutex
)

// GetKeyboardLayout returns the keyboard layout registered with name.
func GetKeyboardLayout(name string) KeyboardLayout {
	mx.RLock()
	defer mx.RUnlock()
	return kbdLayouts[name]
}

func init() {
	initUS()
}

// Register the given keyboard layout.
// This function panics if a keyboard layout with the same name is already registered.
func register(lang string, validKeys map[KeyInput]bool, keys map[KeyInput]KeyDefinition) {
	mx.Lock()
	defer mx.Unlock()

	if _, ok := kbdLayouts[lang]; ok {
		panic(fmt.Sprintf("keyboard layout already registered: %s", lang))
	}
	kbdLayouts[lang] = KeyboardLayout{ValidKeys: validKeys, Keys: keys}
}
