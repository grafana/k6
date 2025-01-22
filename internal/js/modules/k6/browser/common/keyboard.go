package common

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"

	"go.k6.io/k6/internal/js/modules/k6/browser/keyboardlayout"
)

const (
	ModifierKeyAlt int64 = 1 << iota
	ModifierKeyControl
	ModifierKeyMeta
	ModifierKeyShift
)

// KeyboardOptions represents the options for the keyboard.
type KeyboardOptions struct {
	Delay int64 `json:"delay"`
}

// Keyboard represents a keyboard input device.
// Each Page has a publicly accessible Keyboard.
type Keyboard struct {
	ctx     context.Context
	session session

	modifiers   int64          // like shift, alt, ctrl, ...
	pressedKeys map[int64]bool // tracks keys through down() and up()
	layoutName  string         // us by default
	layout      keyboardlayout.KeyboardLayout
}

// NewKeyboard returns a new keyboard with a "us" layout.
func NewKeyboard(ctx context.Context, s session) *Keyboard {
	return &Keyboard{
		ctx:         ctx,
		session:     s,
		pressedKeys: make(map[int64]bool),
		layoutName:  "us",
		layout:      keyboardlayout.GetKeyboardLayout("us"),
	}
}

// Down sends a key down message to a session target.
func (k *Keyboard) Down(key string) error {
	if err := k.down(key); err != nil {
		return fmt.Errorf("sending key down: %w", err)
	}
	return nil
}

// Up sends a key up message to a session target.
func (k *Keyboard) Up(key string) error {
	if err := k.up(key); err != nil {
		return fmt.Errorf("sending key up: %w", err)
	}
	return nil
}

// Press sends a key press message to a session target.
// It delays the action if `Delay` option is specified.
// A press message consists of successive key down and up messages.
func (k *Keyboard) Press(key string, kbdOpts KeyboardOptions) error {
	if err := k.comboPress(key, kbdOpts); err != nil {
		return fmt.Errorf("pressing key: %w", err)
	}

	return nil
}

// InsertText inserts a text without dispatching key events.
func (k *Keyboard) InsertText(text string) error {
	if err := k.insertText(text); err != nil {
		return fmt.Errorf("inserting text: %w", err)
	}
	return nil
}

// Type sends a press message to a session target for each character in text.
// It delays the action if `Delay` option is specified.
//
// It sends an insertText message if a character is not among
// valid characters in the keyboard's layout.
func (k *Keyboard) Type(text string, kbdOpts KeyboardOptions) error {
	if err := k.typ(text, kbdOpts); err != nil {
		return fmt.Errorf("typing text: %w", err)
	}
	return nil
}

func (k *Keyboard) down(key string) error {
	key = k.platformSpecificResolution(key)

	keyInput := keyboardlayout.KeyInput(key)
	if _, ok := k.layout.ValidKeys[keyInput]; !ok {
		return fmt.Errorf("%q is not a valid key for layout %q", key, k.layoutName)
	}

	keyDef := k.keyDefinitionFromKey(keyInput)
	k.modifiers |= k.modifierBitFromKeyName(keyDef.Key)
	text := keyDef.Text
	_, autoRepeat := k.pressedKeys[keyDef.KeyCode]
	k.pressedKeys[keyDef.KeyCode] = true

	keyType := input.KeyDown
	if text == "" {
		keyType = input.KeyRawDown
	}

	action := input.DispatchKeyEvent(keyType).
		WithModifiers(input.Modifier(k.modifiers)).
		WithKey(keyDef.Key).
		WithWindowsVirtualKeyCode(keyDef.KeyCode).
		WithCode(keyDef.Code).
		WithLocation(keyDef.Location).
		WithIsKeypad(keyDef.Location == 3).
		WithText(text).
		WithUnmodifiedText(text).
		WithAutoRepeat(autoRepeat)
	if err := action.Do(cdp.WithExecutor(k.ctx, k.session)); err != nil {
		return fmt.Errorf("dispatching key event down: %w", err)
	}

	return nil
}

func (k *Keyboard) up(key string) error {
	key = k.platformSpecificResolution(key)

	keyInput := keyboardlayout.KeyInput(key)
	if _, ok := k.layout.ValidKeys[keyInput]; !ok {
		return fmt.Errorf("'%s' is not a valid key for layout '%s'", key, k.layoutName)
	}

	keyDef := k.keyDefinitionFromKey(keyInput)
	k.modifiers &= ^k.modifierBitFromKeyName(keyDef.Key)
	delete(k.pressedKeys, keyDef.KeyCode)

	action := input.DispatchKeyEvent(input.KeyUp).
		WithModifiers(input.Modifier(k.modifiers)).
		WithKey(keyDef.Key).
		WithWindowsVirtualKeyCode(keyDef.KeyCode).
		WithCode(keyDef.Code).
		WithLocation(keyDef.Location)
	if err := action.Do(cdp.WithExecutor(k.ctx, k.session)); err != nil {
		return fmt.Errorf("dispatching key event up: %w", err)
	}

	return nil
}

func (k *Keyboard) insertText(text string) error {
	action := input.InsertText(text)
	if err := action.Do(cdp.WithExecutor(k.ctx, k.session)); err != nil {
		return fmt.Errorf("inserting text: %w", err)
	}
	return nil
}

func (k *Keyboard) keyDefinitionFromKey(key keyboardlayout.KeyInput) keyboardlayout.KeyDefinition {
	shift := k.modifiers & ModifierKeyShift

	// Find directly from the keyboard layout
	srcKeyDef, ok := k.layout.Keys[key]
	// Try to find based on key value instead of code
	if !ok {
		srcKeyDef, ok = k.layout.KeyDefinition(key)
	}
	// Try to find with the shift key value
	// e.g. for `@`, the shift modifier needs to
	// be used.
	var foundInShift bool
	if !ok {
		srcKeyDef = k.layout.ShiftKeyDefinition(key)
		shift = k.modifiers | ModifierKeyShift
		foundInShift = true
	}

	var keyDef keyboardlayout.KeyDefinition
	keyDef.Code = srcKeyDef.Code
	if srcKeyDef.Key != "" {
		keyDef.Key = srcKeyDef.Key
	}
	if len(srcKeyDef.Key) == 1 {
		keyDef.Text = srcKeyDef.Key
	}
	if shift != 0 && srcKeyDef.ShiftKeyCode != 0 {
		keyDef.KeyCode = srcKeyDef.ShiftKeyCode
	}
	if srcKeyDef.KeyCode != 0 {
		keyDef.KeyCode = srcKeyDef.KeyCode
	}
	if srcKeyDef.Location != 0 {
		keyDef.Location = srcKeyDef.Location
	}
	if srcKeyDef.Text != "" {
		keyDef.Text = srcKeyDef.Text
	}
	// Shift is only used on keys which are `KeyX`` (where X is
	// A-Z), or on keys which require shift to be pressed e.g.
	// `@`, and shift must be pressed as well as a shiftKey
	// text value present for the key.
	// Not all keys have a text value when shift is pressed
	// e.g. `Control`.
	// When a key such as `2` is pressed, we must ignore shift
	// otherwise we would type `@`.
	isKeyXOrOnShiftLayerAndShiftUsed := (strings.HasPrefix(string(key), "Key") || foundInShift) &&
		shift != 0 &&
		srcKeyDef.ShiftKey != ""
	if isKeyXOrOnShiftLayerAndShiftUsed {
		keyDef.Key = srcKeyDef.ShiftKey
		keyDef.Text = srcKeyDef.ShiftKey
	}
	// If any modifiers besides shift are pressed, no text should be sent
	if k.modifiers & ^ModifierKeyShift != 0 {
		keyDef.Text = ""
	}
	return keyDef
}

func (k *Keyboard) modifierBitFromKeyName(key string) int64 {
	switch key {
	case "Alt":
		return ModifierKeyAlt
	case "Control":
		return ModifierKeyControl
	case "Meta":
		return ModifierKeyMeta
	case "Shift":
		return ModifierKeyShift
	}
	return 0
}

func (k *Keyboard) platformSpecificResolution(key string) string {
	if key == "ControlOrMeta" {
		if runtime.GOOS == "darwin" {
			key = "Meta"
		} else {
			key = "Control"
		}
	}
	return key
}

func (k *Keyboard) comboPress(keys string, opts KeyboardOptions) error {
	if opts.Delay > 0 {
		if err := wait(k.ctx, opts.Delay); err != nil {
			return err
		}
	}

	kk := split(keys)
	for _, key := range kk {
		if err := k.down(key); err != nil {
			return fmt.Errorf("cannot do key down: %w", err)
		}
	}

	for i := range kk {
		key := kk[len(kk)-i-1]
		if err := k.up(key); err != nil {
			return fmt.Errorf("cannot do key up: %w", err)
		}
	}

	return nil
}

// This splits the string on `+`.
// If `+` on it's own is passed, it will return ["+"].
// If `++` is passed in, it will return ["+", ""].
// If `+++` is passed in, it will return ["+", "+"].
func split(keys string) []string {
	var (
		kk = make([]string, 0)
		s  strings.Builder
	)
	for _, r := range keys {
		if r == '+' && s.Len() > 0 {
			kk = append(kk, s.String())
			s.Reset()
		} else {
			s.WriteRune(r)
		}
	}
	kk = append(kk, s.String())

	return kk
}

func (k *Keyboard) press(key string, opts KeyboardOptions) error {
	if opts.Delay > 0 {
		if err := wait(k.ctx, opts.Delay); err != nil {
			return err
		}
	}
	if err := k.down(key); err != nil {
		return fmt.Errorf("key down: %w", err)
	}
	return k.up(key)
}

func (k *Keyboard) typ(text string, opts KeyboardOptions) error {
	layout := keyboardlayout.GetKeyboardLayout(k.layoutName)
	for _, c := range text {
		if opts.Delay > 0 {
			if err := wait(k.ctx, opts.Delay); err != nil {
				return err
			}
		}
		keyInput := keyboardlayout.KeyInput(c)
		if _, ok := layout.ValidKeys[keyInput]; ok {
			if err := k.press(string(c), opts); err != nil {
				return fmt.Errorf("pressing key: %w", err)
			}
			continue
		}
		if err := k.insertText(string(c)); err != nil {
			return fmt.Errorf("inserting text: %w", err)
		}
	}
	return nil
}

func wait(ctx context.Context, delay int64) error {
	t := time.NewTimer(time.Duration(delay) * time.Millisecond)
	select {
	case <-ctx.Done():
		if !t.Stop() {
			<-t.C
		}
		return fmt.Errorf("%w", ctx.Err())
	case <-t.C:
	}

	return nil
}
