package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
)

func syncMapKeyboard(vu moduleVU, kb *common.Keyboard) mapping {
	return mapping{
		"down": kb.Down,
		"up":   kb.Up,
		"press": func(key string, opts sobek.Value) error {
			kbdOpts := common.NewKeyboardOptions()
			if err := kbdOpts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing keyboard options: %w", err)
			}

			return kb.Press(key, kbdOpts) //nolint:wrapcheck
		},
		"type": func(text string, opts sobek.Value) error {
			kbdOpts := common.NewKeyboardOptions()
			if err := kbdOpts.Parse(vu.Context(), opts); err != nil {
				return fmt.Errorf("parsing keyboard options: %w", err)
			}

			return kb.Type(text, kbdOpts) //nolint:wrapcheck
		},
		"insertText": kb.InsertText,
	}
}
