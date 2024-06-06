// Package grpc is the root module of the k6-grpc extension.
package grpc

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"google.golang.org/grpc/codes"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the GRPC module for every VU.
	ModuleInstance struct {
		vu      modules.VU
		exports map[string]interface{}
		metrics *instanceMetrics
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	metrics, err := registerMetrics(vu.InitEnv().Registry)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("failed to register GRPC module metrics: %w", err))
	}

	mi := &ModuleInstance{
		vu:      vu,
		exports: make(map[string]interface{}),
		metrics: metrics,
	}

	mi.exports["Client"] = mi.NewClient
	mi.defineConstants()
	mi.exports["Stream"] = mi.stream

	return mi
}

// NewClient is the JS constructor for the grpc Client.
func (mi *ModuleInstance) NewClient(_ sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	return rt.ToValue(&Client{vu: mi.vu}).ToObject(rt)
}

// defineConstants defines the constant variables of the module.
func (mi *ModuleInstance) defineConstants() {
	rt := mi.vu.Runtime()
	mustAddCode := func(name string, code codes.Code) {
		mi.exports[name] = rt.ToValue(code)
	}

	mustAddCode("StatusOK", codes.OK)
	mustAddCode("StatusCanceled", codes.Canceled)
	mustAddCode("StatusUnknown", codes.Unknown)
	mustAddCode("StatusInvalidArgument", codes.InvalidArgument)
	mustAddCode("StatusDeadlineExceeded", codes.DeadlineExceeded)
	mustAddCode("StatusNotFound", codes.NotFound)
	mustAddCode("StatusAlreadyExists", codes.AlreadyExists)
	mustAddCode("StatusPermissionDenied", codes.PermissionDenied)
	mustAddCode("StatusResourceExhausted", codes.ResourceExhausted)
	mustAddCode("StatusFailedPrecondition", codes.FailedPrecondition)
	mustAddCode("StatusAborted", codes.Aborted)
	mustAddCode("StatusOutOfRange", codes.OutOfRange)
	mustAddCode("StatusUnimplemented", codes.Unimplemented)
	mustAddCode("StatusInternal", codes.Internal)
	mustAddCode("StatusUnavailable", codes.Unavailable)
	mustAddCode("StatusDataLoss", codes.DataLoss)
	mustAddCode("StatusUnauthenticated", codes.Unauthenticated)
}

// Exports returns the exports of the grpc module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: mi.exports,
	}
}

// stream returns a new stream object
func (mi *ModuleInstance) stream(c sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()

	client, err := extractClient(c.Argument(0), rt)
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid GRPC Stream's client: %w", err))
	}

	methodName := sanitizeMethodName(c.Argument(1).String())
	methodDescriptor, err := client.getMethodDescriptor(methodName)
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid GRPC Stream's method: %w", err))
	}

	p, err := newCallParams(mi.vu, c.Argument(2))
	if err != nil {
		common.Throw(rt, fmt.Errorf("invalid GRPC Stream's parameters: %w", err))
	}

	p.SetSystemTags(mi.vu.State(), client.addr, methodName)

	logger := mi.vu.State().Logger.WithField("streamMethod", methodName)

	s := &stream{
		vu:               mi.vu,
		client:           client,
		methodDescriptor: methodDescriptor,
		method:           methodName,
		logger:           logger,

		tq: taskqueue.New(mi.vu.RegisterCallback),

		instanceMetrics: mi.metrics,
		builtinMetrics:  mi.vu.State().BuiltinMetrics,
		done:            make(chan struct{}),
		writingState:    opened,

		writeQueueCh: make(chan message),

		eventListeners: newEventListeners(),
		obj:            rt.NewObject(),
		tagsAndMeta:    &p.TagsAndMeta,
	}

	defineStream(rt, s)

	err = s.beginStream(p)
	if err != nil {
		s.tq.Close()

		common.Throw(rt, err)
	}

	return s.obj
}

// extractClient extracts & validates a grpc.Client from a sobek.Value.
func extractClient(v sobek.Value, rt *sobek.Runtime) (*Client, error) {
	if common.IsNullish(v) {
		return nil, errors.New("empty gRPC client")
	}

	client, ok := v.ToObject(rt).Export().(*Client)
	if !ok {
		return nil, errors.New("not a gRPC client")
	}

	if client.conn == nil {
		return nil, errors.New("no gRPC connection, you must call connect first")
	}

	return client, nil
}
