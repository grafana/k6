package common

import (
	"context"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/k6ext"
)

// KeyboardOptions represents the options for the keyboard.
type KeyboardOptions struct {
	Delay int64 `json:"delay"`
}

// Parse parses the keyboard options.
func (o *KeyboardOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			}
		}
	}

	return nil
}
