package common

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/sobek"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// GetByRoleOptions are the optional options fow when working with the
// GetByRole API.
type GetByRoleOptions struct {
	Checked       *bool   `json:"checked"`
	Disabled      *bool   `json:"disabled"`
	Exact         *bool   `json:"exact"`
	Expanded      *bool   `json:"expanded"`
	IncludeHidden *bool   `json:"includeHidden"`
	Level         *int64  `json:"level"`
	Name          *string `json:"name"`
	Pressed       *bool   `json:"pressed"`
	Selected      *bool   `json:"selected"`
}

// NewGetByRoleOptions will create a new empty GetByRoleOptions instance.
func NewGetByRoleOptions() *GetByRoleOptions {
	return &GetByRoleOptions{}
}

// Parse parses the GetByRole options from the Sobek.Value.
func (o *GetByRoleOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "checked":
			val := obj.Get(k).ToBoolean()
			o.Checked = &val
		case "disabled":
			val := obj.Get(k).ToBoolean()
			o.Disabled = &val
		case "exact":
			val := obj.Get(k).ToBoolean()
			o.Exact = &val
		case "expanded":
			val := obj.Get(k).ToBoolean()
			o.Expanded = &val
		case "includeHidden":
			val := obj.Get(k).ToBoolean()
			o.IncludeHidden = &val
		case "level":
			val := obj.Get(k).ToInteger()
			o.Level = &val
		case "name":
			var val string
			switch obj.Get(k).ExportType() {
			case reflect.TypeOf(string("")):
				val = fmt.Sprintf("'%s'", obj.Get(k).String()) // Strings require quotes
			case reflect.TypeOf(map[string]interface{}(nil)): // JS RegExp
				val = obj.Get(k).String() // No quotes
			default: // CSS, numbers or booleans
				val = obj.Get(k).String() // No quotes
			}
			o.Name = &val
		case "pressed":
			val := obj.Get(k).ToBoolean()
			o.Pressed = &val
		case "selected":
			val := obj.Get(k).ToBoolean()
			o.Selected = &val
		}
	}

	return nil
}
