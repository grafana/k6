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
