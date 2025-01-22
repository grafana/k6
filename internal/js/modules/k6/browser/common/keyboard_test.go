package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/internal/js/modules/k6/browser/keyboardlayout"
)

func TestSplit(t *testing.T) {
	t.Parallel()

	type args struct {
		keys string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty slice on empty string",
			args: args{
				keys: "",
			},
			want: []string{""},
		},
		{
			name: "empty slice on string without separator",
			args: args{
				keys: "HelloWorld!",
			},
			want: []string{"HelloWorld!"},
		},
		{
			name: "string split with separator",
			args: args{
				keys: "Hello+World+!",
			},
			want: []string{"Hello", "World", "!"},
		},
		{
			name: "do not split on single +",
			args: args{
				keys: "+",
			},
			want: []string{"+"},
		},
		{
			name: "split ++ to + and ''",
			args: args{
				keys: "++",
			},
			want: []string{"+", ""},
		},
		{
			name: "split +++ to + and +",
			args: args{
				keys: "+++",
			},
			want: []string{"+", "+"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := split(tt.args.keys)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKeyboardPress(t *testing.T) {
	t.Parallel()

	t.Run("panics when '' empty key passed in", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		k := NewKeyboard(vu.Context(), nil)
		require.Error(t, k.Press("", KeyboardOptions{}))
	})
}

func TestKeyDefinitionCode(t *testing.T) {
	t.Parallel()

	var (
		vu = k6test.NewVU(t)
		k  = NewKeyboard(vu.Context(), nil)
	)

	tests := []struct {
		key           keyboardlayout.KeyInput
		expectedCodes []string
	}{
		{key: "Escape", expectedCodes: []string{"Escape"}},
		{key: "F1", expectedCodes: []string{"F1"}},
		{key: "F2", expectedCodes: []string{"F2"}},
		{key: "F3", expectedCodes: []string{"F3"}},
		{key: "F4", expectedCodes: []string{"F4"}},
		{key: "F5", expectedCodes: []string{"F5"}},
		{key: "F6", expectedCodes: []string{"F6"}},
		{key: "F7", expectedCodes: []string{"F7"}},
		{key: "F8", expectedCodes: []string{"F8"}},
		{key: "F9", expectedCodes: []string{"F9"}},
		{key: "F10", expectedCodes: []string{"F10"}},
		{key: "F11", expectedCodes: []string{"F11"}},
		{key: "F12", expectedCodes: []string{"F12"}},
		{key: "`", expectedCodes: []string{"Backquote"}},
		{key: "-", expectedCodes: []string{"Minus", "NumpadSubtract"}},
		{key: "=", expectedCodes: []string{"Equal"}},
		{key: "\\", expectedCodes: []string{"Backslash"}},
		{key: "Backspace", expectedCodes: []string{"Backspace"}},
		{key: "Tab", expectedCodes: []string{"Tab"}},
		{key: "q", expectedCodes: []string{"KeyQ"}},
		{key: "w", expectedCodes: []string{"KeyW"}},
		{key: "e", expectedCodes: []string{"KeyE"}},
		{key: "r", expectedCodes: []string{"KeyR"}},
		{key: "t", expectedCodes: []string{"KeyT"}},
		{key: "y", expectedCodes: []string{"KeyY"}},
		{key: "u", expectedCodes: []string{"KeyU"}},
		{key: "i", expectedCodes: []string{"KeyI"}},
		{key: "o", expectedCodes: []string{"KeyO"}},
		{key: "p", expectedCodes: []string{"KeyP"}},
		{key: "[", expectedCodes: []string{"BracketLeft"}},
		{key: "]", expectedCodes: []string{"BracketRight"}},
		{key: "CapsLock", expectedCodes: []string{"CapsLock"}},
		{key: "a", expectedCodes: []string{"KeyA"}},
		{key: "s", expectedCodes: []string{"KeyS"}},
		{key: "d", expectedCodes: []string{"KeyD"}},
		{key: "f", expectedCodes: []string{"KeyF"}},
		{key: "g", expectedCodes: []string{"KeyG"}},
		{key: "h", expectedCodes: []string{"KeyH"}},
		{key: "j", expectedCodes: []string{"KeyJ"}},
		{key: "k", expectedCodes: []string{"KeyK"}},
		{key: "l", expectedCodes: []string{"KeyL"}},
		{key: ";", expectedCodes: []string{"Semicolon"}},
		{key: "'", expectedCodes: []string{"Quote"}},
		{key: "Shift", expectedCodes: []string{"ShiftLeft", "ShiftRight"}},
		{key: "z", expectedCodes: []string{"KeyZ"}},
		{key: "x", expectedCodes: []string{"KeyX"}},
		{key: "c", expectedCodes: []string{"KeyC"}},
		{key: "v", expectedCodes: []string{"KeyV"}},
		{key: "b", expectedCodes: []string{"KeyB"}},
		{key: "n", expectedCodes: []string{"KeyN"}},
		{key: "m", expectedCodes: []string{"KeyM"}},
		{key: ",", expectedCodes: []string{"Comma"}},
		{key: "/", expectedCodes: []string{"Slash", "NumpadDivide"}},
		{key: "Control", expectedCodes: []string{"ControlLeft", "ControlRight"}},
		{key: "Meta", expectedCodes: []string{"MetaLeft", "MetaRight"}},
		{key: "Alt", expectedCodes: []string{"AltLeft", "AltRight"}},
		{key: " ", expectedCodes: []string{"Space"}},
		{key: "AltGraph", expectedCodes: []string{"AltGraph"}},
		{key: "ConTextMenu", expectedCodes: []string{"ConTextMenu"}},
		{key: "PrintScreen", expectedCodes: []string{"PrintScreen"}},
		{key: "ScrollLock", expectedCodes: []string{"ScrollLock"}},
		{key: "Pause", expectedCodes: []string{"Pause"}},
		{key: "PageUp", expectedCodes: []string{"PageUp"}},
		{key: "PageDown", expectedCodes: []string{"PageDown"}},
		{key: "Insert", expectedCodes: []string{"Insert"}},
		{key: "Delete", expectedCodes: []string{"Delete"}},
		{key: "Home", expectedCodes: []string{"Home"}},
		{key: "End", expectedCodes: []string{"End"}},
		{key: "ArrowLeft", expectedCodes: []string{"ArrowLeft"}},
		{key: "ArrowUp", expectedCodes: []string{"ArrowUp"}},
		{key: "ArrowRight", expectedCodes: []string{"ArrowRight"}},
		{key: "ArrowDown", expectedCodes: []string{"ArrowDown"}},
		{key: "NumLock", expectedCodes: []string{"NumLock"}},
		{key: "*", expectedCodes: []string{"NumpadMultiply"}},
		{key: "7", expectedCodes: []string{"Numpad7", "Digit7"}},
		{key: "8", expectedCodes: []string{"Numpad8", "Digit8"}},
		{key: "9", expectedCodes: []string{"Numpad9", "Digit9"}},
		{key: "4", expectedCodes: []string{"Numpad4", "Digit4"}},
		{key: "5", expectedCodes: []string{"Numpad5", "Digit5"}},
		{key: "6", expectedCodes: []string{"Numpad6", "Digit6"}},
		{key: "+", expectedCodes: []string{"NumpadAdd"}},
		{key: "1", expectedCodes: []string{"Numpad1", "Digit1"}},
		{key: "2", expectedCodes: []string{"Numpad2", "Digit2"}},
		{key: "3", expectedCodes: []string{"Numpad3", "Digit3"}},
		{key: "0", expectedCodes: []string{"Numpad0", "Digit0"}},
		{key: ".", expectedCodes: []string{"NumpadDecimal", "Period"}},
		{key: "Enter", expectedCodes: []string{"NumpadEnter", "Enter"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.key), func(t *testing.T) {
			t.Parallel()

			kd := k.keyDefinitionFromKey(tt.key)
			assert.Contains(t, tt.expectedCodes, kd.Code)
		})
	}
}
