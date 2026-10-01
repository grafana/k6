package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

func mapKeyboard(vu moduleVU, kb *common.Keyboard) mapping {
	return finishMapping(newKeyboardMapping(vu, kb))
}

func newKeyboardMapping(vu moduleVU, kb *common.Keyboard) mapping {
	return mapping{
		"down": networkCall(func(key string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, kb.Down(key) //nolint:wrapcheck
			})
		}),
		"up": networkCall(func(key string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, kb.Up(key) //nolint:wrapcheck
			})
		}),
		"press": networkCall(func(key string, opts sobek.Value) (*sobek.Promise, error) {
			kbopts, err := exportTo[common.KeyboardOptions](vu.Runtime(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing keyboard options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, kb.Press(key, kbopts)
			}), nil
		}),
		"type": networkCall(func(text string, opts sobek.Value) (*sobek.Promise, error) {
			kbopts, err := exportTo[common.KeyboardOptions](vu.Runtime(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing keyboard options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, kb.Type(text, kbopts)
			}), nil
		}),
		"insertText": networkCall(func(text string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, kb.InsertText(text) //nolint:wrapcheck
			})
		}),
	}
}
