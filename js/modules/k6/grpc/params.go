package grpc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/sobek"
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
	Metadata               metadata.MD
	TagsAndMeta            metrics.TagsAndMeta
	Timeout                time.Duration
	DiscardResponseMessage bool
}

// newCallParams constructs the call parameters from the input value.
// if no input is given, the default values are used.
func newCallParams(vu modules.VU, input sobek.Value) (*callParams, error) {
	result := &callParams{
		Metadata:    metadata.New(nil),
		TagsAndMeta: vu.State().Tags.GetCurrentValues(),
	}

	if common.IsNullish(input) {
		return result, nil
	}

	rt := vu.Runtime()
	params := input.ToObject(rt)

	for _, k := range params.Keys() {
		switch k {
		case "metadata":
			md, err := newMetadata(params.Get(k))
			if err != nil {
				return result, fmt.Errorf("invalid metadata param: %w", err)
			}

			result.Metadata = md
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
		case "discardResponseMessage":
			result.DiscardResponseMessage = params.Get(k).ToBoolean()
		default:
			return result, fmt.Errorf("unknown param: %q", k)
		}
	}

	return result, nil
}

// newMetadata constructs a metadata.MD from the input value.
func newMetadata(input sobek.Value) (metadata.MD, error) {
	md := metadata.New(nil)

	if common.IsNullish(input) {
		return md, nil
	}

	v := input.Export()

	rawHeaders, ok := v.(map[string]interface{})
	if !ok {
		return md, errors.New("must be an object with key-value pairs")
	}

	for hk, kv := range rawHeaders {
		var val string
		// The gRPC spec defines that Binary-valued keys end in -bin
		// https://grpc.io/docs/what-is-grpc/core-concepts/#metadata
		if strings.HasSuffix(hk, "-bin") {
			var binVal []byte
			if binVal, ok = kv.([]byte); !ok {
				return md, fmt.Errorf("%q value must be binary", hk)
			}

			// https://github.com/grpc/grpc-go/blob/v1.57.0/Documentation/grpc-metadata.md#storing-binary-data-in-metadata
			val = string(binVal)
		} else if val, ok = kv.(string); !ok {
			return md, fmt.Errorf("%q value must be a string", hk)
		}

		md.Append(hk, val)
	}

	return md, nil
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

// connectParams is the parameters that can be passed to a gRPC connect call.
type connectParams struct {
	IsPlaintext           bool
	UseReflectionProtocol bool
	ReflectionMetadata    metadata.MD
	Timeout               time.Duration
	MaxReceiveSize        int64
	MaxSendSize           int64
	TLS                   map[string]interface{}
}

func newConnectParams(vu modules.VU, input sobek.Value) (*connectParams, error) { //nolint:gocognit
	result := &connectParams{
		IsPlaintext:           false,
		UseReflectionProtocol: false,
		Timeout:               time.Minute,
		MaxReceiveSize:        0,
		MaxSendSize:           0,
		ReflectionMetadata:    metadata.New(nil),
	}

	if common.IsNullish(input) {
		return result, nil
	}

	rt := vu.Runtime()
	params := input.ToObject(rt)

	for _, k := range params.Keys() {
		v := params.Get(k).Export()

		switch k {
		case "plaintext":
			var ok bool
			result.IsPlaintext, ok = v.(bool)
			if !ok {
				return result, fmt.Errorf("invalid plaintext value: '%#v', it needs to be boolean", v)
			}
		case "timeout":
			var err error
			result.Timeout, err = types.GetDurationValue(v)
			if err != nil {
				return result, fmt.Errorf("invalid timeout value: %w", err)
			}
		case "reflect":
			var ok bool
			result.UseReflectionProtocol, ok = v.(bool)
			if !ok {
				return result, fmt.Errorf("invalid reflect value: '%#v', it needs to be boolean", v)
			}
		case "reflectMetadata":
			md, err := newMetadata(params.Get(k))
			if err != nil {
				return result, fmt.Errorf("invalid reflectMetadata param: %w", err)
			}

			result.ReflectionMetadata = md
		case "maxReceiveSize":
			var ok bool
			result.MaxReceiveSize, ok = v.(int64)
			if !ok {
				return result, fmt.Errorf("invalid maxReceiveSize value: '%#v', it needs to be an integer", v)
			}
			if result.MaxReceiveSize < 0 {
				return result, fmt.Errorf("invalid maxReceiveSize value: '%#v, it needs to be a positive integer", v)
			}
		case "maxSendSize":
			var ok bool
			result.MaxSendSize, ok = v.(int64)
			if !ok {
				return result, fmt.Errorf("invalid maxSendSize value: '%#v', it needs to be an integer", v)
			}
			if result.MaxSendSize < 0 {
				return result, fmt.Errorf("invalid maxSendSize value: '%#v, it needs to be a positive integer", v)
			}
		case "tls":
			if err := parseConnectTLSParam(result, v); err != nil {
				return result, err
			}
		default:
			return result, fmt.Errorf("unknown connect param: %q", k)
		}
	}

	return result, nil
}

func parseConnectTLSParam(params *connectParams, v interface{}) error {
	var ok bool
	params.TLS, ok = v.(map[string]interface{})

	if !ok {
		return fmt.Errorf("invalid tls value: '%#v', expected (optional) keys: cert, key, password, and cacerts", v)
	}
	// optional map keys below
	if cert, certok := params.TLS["cert"]; certok {
		if _, ok = cert.(string); !ok {
			return fmt.Errorf("invalid tls cert value: '%#v', it needs to be a PEM formatted string", v)
		}
	}
	if key, keyok := params.TLS["key"]; keyok {
		if _, ok = key.(string); !ok {
			return fmt.Errorf("invalid tls key value: '%#v', it needs to be a PEM formatted string", v)
		}
	}
	if pass, passok := params.TLS["password"]; passok {
		if _, ok = pass.(string); !ok {
			return fmt.Errorf("invalid tls password value: '%#v', it needs to be a string", v)
		}
	}
	if cacerts, cacertsok := params.TLS["cacerts"]; cacertsok {
		var cacertsArray []interface{}
		if cacertsArray, ok = cacerts.([]interface{}); ok {
			for _, cacertsArrayEntry := range cacertsArray {
				if _, ok = cacertsArrayEntry.(string); !ok {
					return fmt.Errorf("invalid tls cacerts value: '%#v',"+
						" it needs to be a string or an array of PEM formatted strings", v)
				}
			}
		} else if _, ok = cacerts.(string); !ok {
			return fmt.Errorf("invalid tls cacerts value: '%#v',"+
				" it needs to be a string or an array of PEM formatted strings", v)
		}
	}
	return nil
}
