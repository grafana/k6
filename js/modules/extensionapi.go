package modules

import (
	"context"
	"net"
	"sync"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

// extensionAPIModuleAdapter is the k6-owned translation layer between the
// standalone extension API and the legacy module resolver. No k6 type is
// exposed from the standalone API.
type extensionAPIModuleAdapter struct {
	module extensionapi.Module
}

func (a extensionAPIModuleAdapter) NewModuleInstance(vu VU) Instance {
	return extensionAPIInstanceAdapter{instance: a.module.NewModuleInstance(extensionAPIVU{vu: vu})}
}

type extensionAPIVU struct {
	vu VU
}

func (v extensionAPIVU) Context() context.Context {
	return v.vu.Context()
}

func (v extensionAPIVU) Runtime() *sobek.Runtime {
	return v.vu.Runtime()
}

func (v extensionAPIVU) LookupEnv(key string) (string, bool) {
	initEnv := v.vu.InitEnv()
	if initEnv == nil || initEnv.LookupEnv == nil {
		return "", false
	}

	return initEnv.LookupEnv(key)
}

func (v extensionAPIVU) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	state := v.vu.State()
	if state == nil || state.Dialer == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	return state.Dialer.DialContext(ctx, network, address)
}

func (v extensionAPIVU) LookupHost(ctx context.Context, host string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	state := v.vu.State()
	if state == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	resolver := state.GetAddrResolver()
	if resolver == nil {
		return nil, extensionapi.ErrNetworkUnavailable
	}

	ip, _, err := resolver.ResolveAddr(host)
	if err != nil {
		return nil, err
	}

	return []string{ip.String()}, nil
}

func (v extensionAPIVU) RegisterCallback() func(extensionapi.Task) {
	callback := v.vu.RegisterCallback()
	return func(task extensionapi.Task) {
		callback(func() error { return task() })
	}
}

func (v extensionAPIVU) NewPromise() (*sobek.Promise, extensionapi.PromiseResolver) {
	promise, resolve, reject := v.vu.Runtime().NewPromise()
	return promise, &extensionAPIPromiseResolver{
		enqueue: v.RegisterCallback(),
		resolve: resolve,
		reject:  reject,
	}
}

type extensionAPIPromiseResolver struct {
	once    sync.Once
	enqueue func(extensionapi.Task)
	resolve func(any) error
	reject  func(any) error
}

func (r *extensionAPIPromiseResolver) Resolve(value any) {
	r.once.Do(func() {
		r.enqueue(func() error { return r.resolve(value) })
	})
}

func (r *extensionAPIPromiseResolver) Reject(reason any) {
	r.once.Do(func() {
		r.enqueue(func() error { return r.reject(reason) })
	})
}

type extensionAPIInstanceAdapter struct {
	instance extensionapi.Instance
}

func (a extensionAPIInstanceAdapter) Exports() Exports {
	exports := a.instance.Exports()
	return Exports{Default: exports.Default, Named: exports.Named}
}
