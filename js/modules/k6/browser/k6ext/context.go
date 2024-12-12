package k6ext

import (
	"context"

	k6modules "go.k6.io/k6/js/modules"
	k6lib "go.k6.io/k6/lib"

	"github.com/grafana/sobek"
)

type ctxKey int

const (
	ctxKeyVU ctxKey = iota
	ctxKeyPid
	ctxKeyCustomK6Metrics
)

// WithVU returns a new context based on ctx with the k6 VU instance attached.
func WithVU(ctx context.Context, vu k6modules.VU) context.Context {
	return context.WithValue(ctx, ctxKeyVU, vu)
}

// GetVU returns the attached k6 VU instance from ctx, which can be used to
// retrieve the sobek runtime and other k6 objects relevant to the currently
// executing VU.
// See https://github.com/grafana/k6/blob/v0.37.0/js/initcontext.go#L168-L186
func GetVU(ctx context.Context) k6modules.VU {
	v := ctx.Value(ctxKeyVU)
	if vu, ok := v.(k6modules.VU); ok {
		return vu
	}
	return nil
}

// WithCustomMetrics attaches the CustomK6Metrics object to the context.
func WithCustomMetrics(ctx context.Context, k6m *CustomMetrics) context.Context {
	return context.WithValue(ctx, ctxKeyCustomK6Metrics, k6m)
}

// GetCustomMetrics returns the CustomK6Metrics object attached to the context.
func GetCustomMetrics(ctx context.Context) *CustomMetrics {
	v := ctx.Value(ctxKeyCustomK6Metrics)
	if k6m, ok := v.(*CustomMetrics); ok {
		return k6m
	}
	return nil
}

// Runtime is a convenience function for getting a k6 VU runtime.
func Runtime(ctx context.Context) *sobek.Runtime {
	return GetVU(ctx).Runtime()
}

// GetScenarioName returns the scenario name associated with the given context.
func GetScenarioName(ctx context.Context) string {
	ss := k6lib.GetScenarioState(ctx)
	if ss == nil {
		return ""
	}
	return ss.Name
}

// GetScenarioOpts returns the browser options and environment variables associated
// with the given context.
func GetScenarioOpts(ctx context.Context, vu k6modules.VU) map[string]any {
	scenario := GetScenarioName(ctx)
	if scenario == "" {
		return nil
	}
	if so := vu.State().Options.Scenarios[scenario].GetScenarioOptions(); so != nil {
		return so.Browser
	}
	return nil
}
