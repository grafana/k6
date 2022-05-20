package k6

import (
	"context"

	k6modules "go.k6.io/k6/js/modules"

	"github.com/dop251/goja"
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
// retrieve the goja runtime and other k6 objects relevant to the currently
// executing VU.
// See https://github.com/grafana/k6/blob/v0.37.0/js/initcontext.go#L168-L186
func GetVU(ctx context.Context) k6modules.VU {
	v := ctx.Value(ctxKeyVU)
	if vu, ok := v.(k6modules.VU); ok {
		return vu
	}
	return nil
}

// WithProcessID saves the browser process ID to the context.
func WithProcessID(ctx context.Context, pid int) context.Context {
	return context.WithValue(ctx, ctxKeyPid, pid)
}

// GetProcessID returns the browser process ID from the context.
func GetProcessID(ctx context.Context) int {
	v, _ := ctx.Value(ctxKeyPid).(int)
	return v // it will return zero on error
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
func Runtime(ctx context.Context) *goja.Runtime {
	return GetVU(ctx).Runtime()
}
