/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
// It an empty key definition if it cannot find the key.
func (kl KeyboardLayout) ShiftKeyDefinition(key KeyInput) (KeyInput, KeyDefinition) {
	for k, d := range kl.Keys {
		if d.ShiftKey == string(key) {
			return k, d
		}
	}
	return key, KeyDefinition{}
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
