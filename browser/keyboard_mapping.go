package browser

import (
	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

func mapKeyboard(vu moduleVU, kb *common.Keyboard) mapping {
	return mapping{
		"down": func(key string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, kb.Down(key) //nolint:wrapcheck
			})
		},
		"up": func(key string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, kb.Up(key) //nolint:wrapcheck
			})
		},
		"press": func(key string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, kb.Press(key, opts) //nolint:wrapcheck
			})
		},
		"type": func(text string, opts goja.Value) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, kb.Type(text, opts) //nolint:wrapcheck
			})
		},
		"insertText": func(text string) *goja.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, kb.InsertText(text) //nolint:wrapcheck
			})
		},
	}
}
