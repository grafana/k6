package tracing

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// Tracing is the JS module instance that will be created for each VU.
type Tracing struct {
	vu modules.VU

	defaultClient *Client
}

// NewClient is the JS constructor for the tracing.Client
//
// It expects a single configuration object as argument, which
// will be used to instantiate an `Object` instance internally,
// and will be used by the client to configure itself.
func (t *Tracing) NewClient(cc goja.ConstructorCall) *goja.Object {
	rt := t.vu.Runtime()

	if len(cc.Arguments) < 1 {
		common.Throw(rt, errors.New("Client constructor expects a single configuration object as argument; none given"))
	}

	var opts options
	if err := rt.ExportTo(cc.Arguments[0], &opts); err != nil {
		common.Throw(rt, fmt.Errorf("unable to parse options object; reason: %w", err))
	}

	return rt.ToValue(NewClient(t.vu, opts)).ToObject(rt)
}

// InstrumentHTTP instruments the HTTP module with tracing headers.
//
// When used in the context of a k6 script, it will automatically replace
// the imported http module's methods with instrumented ones.
func (t *Tracing) InstrumentHTTP(options options) {
	rt := t.vu.Runtime()

	// Initialize the tracing module's instance default client,
	// and configure it using the user-supplied set of options.
	t.defaultClient = NewClient(t.vu, options)

	// Explicitly inject the http module in the VU's runtime.
	// This allows us to later on override the http module's methods
	// with instrumented ones.
	httpModuleValue, err := rt.RunString(`require('k6/http')`)
	if err != nil {
		common.Throw(rt, err)
	}

	httpModuleObj := httpModuleValue.ToObject(rt)

	// Closure overriding a method of the provided imported module object.
	//
	// The `onModule` argument should be a *goja.Object obtained by requiring
	// or importing the 'k6/http' module and converting it to an object.
	//
	// The `value` argument is expected to be callable.
	mustSetHTTPMethod := func(method string, onModule *goja.Object, value interface{}) {
		// Inject the new get function, adding tracing headers
		// to the request in the HTTP module object.
		err = onModule.Set(method, value)
		if err != nil {
			common.Throw(
				rt,
				fmt.Errorf("unable to overwrite http.%s method with instrumented one; reason: %w", method, err),
			)
		}
	}

	// Overwrite the implementation of the http module's method with the instrumented
	// ones exposed by the `tracing.Client` struct.
	mustSetHTTPMethod("del", httpModuleObj, t.defaultClient.Delete)
	mustSetHTTPMethod("get", httpModuleObj, t.defaultClient.Get)
	mustSetHTTPMethod("head", httpModuleObj, t.defaultClient.Head)
	mustSetHTTPMethod("options", httpModuleObj, t.defaultClient.Options)
	mustSetHTTPMethod("patch", httpModuleObj, t.defaultClient.Patch)
	mustSetHTTPMethod("post", httpModuleObj, t.defaultClient.Patch)
	mustSetHTTPMethod("put", httpModuleObj, t.defaultClient.Patch)
	mustSetHTTPMethod("request", httpModuleObj, t.defaultClient.Request)

	// Inject the updated HTTP module object in the runtime,
	// overriding any previously imported one in the process.
	err = rt.Set("http", httpModuleObj)
	if err != nil {
		common.Throw(rt, err)
	}
}
