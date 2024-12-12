package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/modules/k6/browser/common"
)

func syncMapKeyboard(vu moduleVU, kb *common.Keyboard) mapping {
	return mapping{
		"down": kb.Down,
		"up":   kb.Up,
		"press": func(key string, opts sobek.Value) error {
			kbopts, err := exportTo[common.KeyboardOptions](vu.Runtime(), opts)
			if err != nil {
				return fmt.Errorf("parsing keyboard options: %w", err)
			}
			return kb.Press(key, kbopts)
		},
		"type": func(text string, opts sobek.Value) error {
			kbopts, err := exportTo[common.KeyboardOptions](vu.Runtime(), opts)
			if err != nil {
				return fmt.Errorf("parsing keyboard options: %w", err)
			}
			return kb.Type(text, kbopts)
		},
		"insertText": kb.InsertText,
	}
}
