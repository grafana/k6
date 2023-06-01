package grpc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc/metadata"
)

// callParams is the parameters that can be passed to a gRPC calls
// like invoke or newStream.
type callParams struct {
	Metadata    metadata.MD
	TagsAndMeta metrics.TagsAndMeta
	Timeout     time.Duration
}

// newCallParams constructs the call parameters from the input value.
// if no input is given, the default values are used.
func newCallParams(vu modules.VU, input goja.Value) (*callParams, error) {
	result := &callParams{
		Metadata:    metadata.New(nil),
		TagsAndMeta: vu.State().Tags.GetCurrentValues(),
	}

	if input == nil || goja.IsUndefined(input) || goja.IsNull(input) {
		return result, nil
	}

	rt := vu.Runtime()
	params := input.ToObject(rt)

	for _, k := range params.Keys() {
		switch k {
		case "metadata":
			v := params.Get(k).Export()
			rawHeaders, ok := v.(map[string]interface{})
			if !ok {
				return result, errors.New("metadata must be an object with key-value pairs")
			}
			for hk, kv := range rawHeaders {
				strval, ok := kv.(string)
				if !ok {
					return result, fmt.Errorf("metadata %q value must be a string", hk)
				}

				result.Metadata.Append(hk, strval)
			}
		case "tags":
			if err := common.ApplyCustomUserTags(rt, &result.TagsAndMeta, params.Get(k)); err != nil {
				return result, fmt.Errorf("metric tags: %w", err)
			}
		case "timeout":
			var err error
			v := params.Get(k).Export()
			result.Timeout, err = types.GetDurationValue(v)
			if err != nil {
				return result, fmt.Errorf("invalid timeout value: %w", err)
			}
		default:
			return result, fmt.Errorf("unknown param: %q", k)
		}
	}

	return result, nil
}

// SetSystemTags sets the system tags for the call.
func (p *callParams) SetSystemTags(state *lib.State, addr string, methodName string) {
	if state.Options.SystemTags.Has(metrics.TagURL) {
		p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagURL, fmt.Sprintf("%s%s", addr, methodName))
	}

	parts := strings.Split(methodName[1:], "/")
	p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagService, parts[0])
	p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagMethod, parts[1])

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := p.TagsAndMeta.Tags.Get("name"); !ok {
		p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagName, methodName)
	}
}
