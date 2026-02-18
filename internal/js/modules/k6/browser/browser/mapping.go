package browser

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"

	k6common "go.k6.io/k6/js/common"
)

// mapping is a type for mapping our module API to sobek.
// It acts like a bridge and allows adding wildcard methods
// and customization over our API.
type mapping = map[string]any

// mapBrowserToSobek maps the browser API to the JS module.
// The motivation of this mapping was to support $ and $$ wildcard
// methods.
// See issue #661 for more details.
func mapBrowserToSobek(vu moduleVU) *sobek.Object {
	var (
		rt  = vu.Runtime()
		obj = rt.NewObject()
	)
	for k, v := range mapBrowser(vu) {
		err := obj.Set(k, rt.ToValue(v))
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("mapping: %w", err))
		}
	}

	return obj
}

func parseFrameClickOptions(
	ctx context.Context, opts sobek.Value, defaultTimeout time.Duration,
) (*common.FrameClickOptions, error) {
	copts := common.NewFrameClickOptions(defaultTimeout)
	if err := copts.Parse(ctx, opts); err != nil {
		return nil, fmt.Errorf("parsing click options: %w", err)
	}
	return copts, nil
}

func ConvertSelectOptionValues(rt *sobek.Runtime, values sobek.Value) ([]any, error) {
	if k6common.IsNullish(values) {
		return nil, nil
	}

	var (
		opts []any
		t    = values.Export()
	)
	switch values.ExportType().Kind() {
	case reflect.Slice:
		var sl []any
		if err := rt.ExportTo(values, &sl); err != nil {
			return nil, fmt.Errorf("options: expected array, got %T", values)
		}

		for _, item := range sl {
			switch item := item.(type) {
			case string:
				// Strings will match values or labels
				valOpt := common.SelectOption{Value: new(string)}
				*valOpt.Value = item
				labelOpt := common.SelectOption{Label: new(string)}
				*labelOpt.Label = item
				opts = append(opts, &valOpt, &labelOpt)
			case map[string]any:
				opt, err := extractSelectOptionFromMap(item)
				if err != nil {
					return nil, err
				}

				opts = append(opts, opt)
			default:
				return nil, fmt.Errorf("options: expected string or object, got %T", item)
			}
		}
	case reflect.Map:
		var raw map[string]any
		if err := rt.ExportTo(values, &raw); err != nil {
			return nil, fmt.Errorf("options: expected object, got %T", values)
		}

		opt, err := extractSelectOptionFromMap(raw)
		if err != nil {
			return nil, err
		}

		opts = append(opts, opt)
	case reflect.TypeFor[*common.ElementHandle]().Kind():
		opts = append(opts, t.(*common.ElementHandle)) //nolint:forcetypeassert
	case reflect.TypeFor[sobek.Object]().Kind():
		obj := values.ToObject(rt)
		opt := common.SelectOption{}
		for _, k := range obj.Keys() {
			switch k {
			case "value":
				opt.Value = new(string)
				*opt.Value = obj.Get(k).String()
			case "label":
				opt.Label = new(string)
				*opt.Label = obj.Get(k).String()
			case "index":
				opt.Index = new(int64)
				*opt.Index = obj.Get(k).ToInteger()
			}
		}
		opts = append(opts, &opt)
	case reflect.String:
		// Strings will match values or labels
		valOpt := common.SelectOption{Value: new(string)}
		*valOpt.Value = t.(string) //nolint:forcetypeassert
		labelOpt := common.SelectOption{Label: new(string)}
		*labelOpt.Label = t.(string) //nolint:forcetypeassert
		opts = append(opts, &valOpt, &labelOpt)
	default:
		return nil, fmt.Errorf("options: unsupported type %T", values)
	}

	return opts, nil
}

func extractSelectOptionFromMap(v map[string]any) (*common.SelectOption, error) {
	opt := &common.SelectOption{}
	for k, raw := range v {
		switch k {
		case "value":
			opt.Value = new(string)

			v, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("options[%v]: expected string, got %T", k, raw)
			}

			*opt.Value = v
		case "label":
			opt.Label = new(string)

			v, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("options[%v]: expected string, got %T", k, raw)
			}
			*opt.Label = v
		case "index":
			opt.Index = new(int64)

			switch raw := raw.(type) {
			case int:
				*opt.Index = int64(raw)
			case int64:
				*opt.Index = raw
			default:
				return nil, fmt.Errorf("options[%v]: expected int, got %T", k, raw)
			}
		}
	}

	return opt, nil
}
