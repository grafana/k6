package js

import (
	"errors"
	"sync"

	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/browser"
	"go.k6.io/k6/internal/js/modules/k6/crypto"
	"go.k6.io/k6/internal/js/modules/k6/crypto/x509"
	"go.k6.io/k6/internal/js/modules/k6/data"
	"go.k6.io/k6/internal/js/modules/k6/encoding"
	"go.k6.io/k6/internal/js/modules/k6/execution"
	"go.k6.io/k6/internal/js/modules/k6/experimental/csv"
	"go.k6.io/k6/internal/js/modules/k6/experimental/fs"
	"go.k6.io/k6/internal/js/modules/k6/experimental/streams"
	expws "go.k6.io/k6/internal/js/modules/k6/experimental/websockets"
	"go.k6.io/k6/internal/js/modules/k6/grpc"
	"go.k6.io/k6/internal/js/modules/k6/metrics"
	"go.k6.io/k6/internal/js/modules/k6/timers"
	"go.k6.io/k6/internal/js/modules/k6/webcrypto"
	"go.k6.io/k6/internal/js/modules/k6/ws"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/modules/k6"
	"go.k6.io/k6/js/modules/k6/html"
	"go.k6.io/k6/js/modules/k6/http"

	"github.com/grafana/xk6-redis/redis"
)

func getInternalJSModules() map[string]interface{} {
	return map[string]interface{}{
		"k6":                      k6.New(),
		"k6/crypto":               crypto.New(),
		"k6/crypto/x509":          x509.New(),
		"k6/data":                 data.New(),
		"k6/encoding":             encoding.New(),
		"k6/timers":               timers.New(),
		"k6/execution":            execution.New(),
		"k6/experimental/csv":     csv.New(),
		"k6/experimental/redis":   redis.New(),
		"k6/experimental/streams": streams.New(),
		"k6/experimental/webcrypto": newWarnExperimentalModule(webcrypto.New(),
			"k6/experimental/webcrypto is now part of the k6 core, and globally available. You could just remove import."+
				" The k6/experimental/webcrypto will be removed in k6 v1.1.0"),
		"k6/experimental/websockets": expws.New(),
		"k6/experimental/timers": newRemovedModule(
			"k6/experimental/timers has been graduated, please use k6/timers instead."),
		"k6/experimental/tracing": newRemovedModule(
			"k6/experimental/tracing has been removed. All of it functionality is available as pure javascript module." +
				" More info available at the docs:" +
				" https://grafana.com/docs/k6/latest/javascript-api/jslib/http-instrumentation-tempo"),
		"k6/experimental/browser": newRemovedModule(
			"k6/experimental/browser has been graduated, please use k6/browser instead." +
				"Please update your imports to use k6/browser instead of k6/experimental/browser," +
				" For more information, see the migration guide at the link:" +
				" https://grafana.com/docs/k6/latest/using-k6-browser/migrating-to-k6-v0-52/"),
		"k6/browser":         browser.New(),
		"k6/experimental/fs": fs.New(),
		"k6/net/grpc":        grpc.New(),
		"k6/html":            html.New(),
		"k6/http":            http.New(),
		"k6/metrics":         metrics.New(),
		"k6/ws":              ws.New(),
		"k6/experimental/grpc": newRemovedModule(
			"k6/experimental/grpc has been graduated, please use k6/net/grpc instead." +
				" See https://grafana.com/docs/k6/latest/javascript-api/k6-net-grpc/ for more information.",
		),
	}
}

func getJSModules() map[string]interface{} {
	result := getInternalJSModules()
	external := ext.Get(ext.JSExtension)

	// external is always prefixed with `k6/x`
	for _, e := range external {
		result[e.Name] = e.Module
	}

	return result
}

type warnExperimentalModule struct {
	once *sync.Once
	msg  string
	base modules.Module
}

func newWarnExperimentalModule(base modules.Module, msg string) modules.Module {
	return &warnExperimentalModule{
		msg:  msg,
		base: base,
		once: &sync.Once{},
	}
}

func (w *warnExperimentalModule) NewModuleInstance(vu modules.VU) modules.Instance {
	w.once.Do(func() { vu.InitEnv().Logger.Warn(w.msg) })
	return w.base.NewModuleInstance(vu)
}

type removedModule struct {
	errMsg string
}

func newRemovedModule(errMsg string) modules.Module {
	return &removedModule{errMsg: errMsg}
}

func (rm *removedModule) NewModuleInstance(vu modules.VU) modules.Instance {
	common.Throw(vu.Runtime(), errors.New(rm.errMsg))

	return nil
}
