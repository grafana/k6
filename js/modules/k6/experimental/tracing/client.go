package tracing

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	httpmodule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/metrics"
)

// Client represents a HTTP Client instrumenting the requests
// it performs with tracing information.
type Client struct {
	vu modules.VU

	// opts holds the client's configuration options.
	opts options

	// propagator holds the client's trace propagator, used
	// to produce trace context headers for each supported
	// formats: w3c, b3, jaeger.
	propagator Propagator

	// requestFunc holds the http module's request function
	// used to emit HTTP requests in k6 script. The client
	// uses it under the hood to emit the requests it
	// instruments.
	requestFunc HTTPRequestFunc

	// asyncRequestFunc holds the http module's asyncRequest function
	// used to emit HTTP requests in k6 script. The client
	// uses it under the hood to emit the requests it
	// instruments.
	asyncRequestFunc HTTPAsyncRequestFunc

	// randSource holds the client's random source, used
	// to generate random values for the trace ID.
	randSource *rand.Rand
}

type (
	// HTTPRequestFunc is a type alias representing the prototype of
	// k6's http module's request function
	HTTPRequestFunc func(method string, url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error)

	// HTTPAsyncRequestFunc is a type alias representing the prototype of
	// k6's http module's asyncRequest function
	HTTPAsyncRequestFunc func(method string, url sobek.Value, args ...sobek.Value) (*sobek.Promise, error)
)

// NewClient instantiates a new tracing Client
func NewClient(vu modules.VU, opts options) (*Client, error) {
	rt := vu.Runtime()

	// Get the http module
	httpModule, err := rt.RunString("require('k6/http')")
	if err != nil {
		return nil,
			fmt.Errorf("failed initializing tracing client, unable to require k6/http module; reason: %w", err)
	}
	httpModuleObject := httpModule.ToObject(rt)

	// Export the http module's request function sobek.Callable as a Go function
	var requestFunc HTTPRequestFunc
	if err := rt.ExportTo(httpModuleObject.Get("request"), &requestFunc); err != nil {
		return nil,
			fmt.Errorf("failed initializing tracing client, unable to require http.request method; reason: %w", err)
	}

	// Export the http module's syncRequest function sobek.Callable as a Go function
	var asyncRequestFunc HTTPAsyncRequestFunc
	if err := rt.ExportTo(httpModuleObject.Get("asyncRequest"), &asyncRequestFunc); err != nil {
		return nil,
			fmt.Errorf("failed initializing tracing client, unable to require http.asyncRequest method; reason: %w",
				err)
	}

	client := &Client{
		vu:               vu,
		requestFunc:      requestFunc,
		asyncRequestFunc: asyncRequestFunc,
		randSource:       rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}

	if err := client.Configure(opts); err != nil {
		return nil,
			fmt.Errorf("failed initializing tracing client, invalid configuration; reason: %w", err)
	}

	return client, nil
}

// Configure configures the tracing client with the given options.
func (c *Client) Configure(opts options) error {
	if err := opts.validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	var sampler Sampler = NewAlwaysOnSampler()
	if opts.Sampling != 1.0 {
		sampler = NewProbabilisticSampler(opts.Sampling)
	}

	switch opts.Propagator {
	case "w3c":
		c.propagator = NewW3CPropagator(sampler)
	case "jaeger":
		c.propagator = NewJaegerPropagator(sampler)
	default:
		return fmt.Errorf("unknown propagator: %s", opts.Propagator)
	}

	c.opts = opts

	return nil
}

// Request instruments the http module's request function with tracing headers,
// and ensures the trace_id is emitted as part of the output's data points metadata.
func (c *Client) Request(method string, url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	var result *httpmodule.Response

	var err error
	err = c.instrumentedCall(func(args ...sobek.Value) error {
		result, err = c.requestFunc(method, url, args...)
		return err
	}, args...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AsyncRequest instruments the http module's asyncRequest function with tracing headers,
// and ensures the trace_id is emitted as part of the output's data points metadata.
func (c *Client) AsyncRequest(method string, url sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
	var result *sobek.Promise
	var err error
	err = c.instrumentedCall(func(args ...sobek.Value) error {
		result, err = c.asyncRequestFunc(method, url, args...)
		return err
	}, args...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Del instruments the http module's delete method.
func (c *Client) Del(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodDelete, url, args...)
}

// Get instruments the http module's get method.
func (c *Client) Get(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	// Here we prepend a null value that stands for the body parameter,
	// that the request function expects as a first argument implicitly
	args = append([]sobek.Value{sobek.Null()}, args...)
	return c.Request(http.MethodGet, url, args...)
}

// Head instruments the http module's head method.
func (c *Client) Head(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	// NB: here we prepend a null value that stands for the body parameter,
	// that the request function expects as a first argument implicitly
	args = append([]sobek.Value{sobek.Null()}, args...)
	return c.Request(http.MethodHead, url, args...)
}

// Options instruments the http module's options method.
func (c *Client) Options(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodOptions, url, args...)
}

// Patch instruments the http module's patch method.
func (c *Client) Patch(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPatch, url, args...)
}

// Post instruments the http module's post method.
func (c *Client) Post(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPost, url, args...)
}

// Put instruments the http module's put method.
func (c *Client) Put(url sobek.Value, args ...sobek.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPut, url, args...)
}

func (c *Client) instrumentedCall(call func(args ...sobek.Value) error, args ...sobek.Value) error {
	if len(args) == 0 {
		args = []sobek.Value{sobek.Null()}
	}

	traceContextHeader, encodedTraceID, err := c.generateTraceContext()
	if err != nil {
		return err
	}

	// update the `params` argument with the trace context header
	// so that it can be used by the http module's request function.
	args, err = c.instrumentArguments(traceContextHeader, args...)
	if err != nil {
		return fmt.Errorf("failed to instrument request arguments; reason: %w", err)
	}

	// Add the trace ID to the VU's state, so that it can be
	// used in the metrics emitted by the HTTP module.
	c.vu.State().Tags.Modify(func(t *metrics.TagsAndMeta) {
		t.SetMetadata(metadataTraceIDKeyName, encodedTraceID)
	})

	// Remove the trace ID from the VU's state, so that it doesn't leak into other requests.
	defer func() {
		c.vu.State().Tags.Modify(func(t *metrics.TagsAndMeta) {
			t.DeleteMetadata(metadataTraceIDKeyName)
		})
	}()

	return call(args...)
}

func (c *Client) generateTraceContext() (http.Header, string, error) {
	traceID, err := newTraceID(k6Prefix, k6CloudCode, time.Now(), c.randSource)
	if err != nil {
		return http.Header{}, "", fmt.Errorf("failed to generate trace ID; reason: %w", err)
	}

	// Produce a trace header in the format defined by the configured propagator.
	traceContextHeader, err := c.propagator.Propagate(traceID)
	if err != nil {
		return http.Header{}, "", fmt.Errorf("failed to propagate trace ID; reason: %w", err)
	}

	return traceContextHeader, traceID, nil
}

// instrumentArguments: expects args to be in the format expected by the
// request method (body, params)
func (c *Client) instrumentArguments(traceContext http.Header, args ...sobek.Value) ([]sobek.Value, error) {
	rt := c.vu.Runtime()

	var paramsObj *sobek.Object

	switch len(args) {
	case 2:
		// We received both a body and a params argument. In the
		// event params would be nullish, we'll instantiate
		// a new object.
		if common.IsNullish(args[1]) {
			paramsObj = rt.NewObject()
			args[1] = paramsObj
		} else {
			paramsObj = args[1].ToObject(rt)
		}
	case 1:
		// We only received a body argument
		paramsObj = rt.NewObject()
		args = append(args, paramsObj)
	default:
		return nil, fmt.Errorf("invalid number of arguments; expected 1 or 2, got %d", len(args))
	}

	headersObj := rt.NewObject()

	headersValue := paramsObj.Get("headers")
	if !common.IsNullish(headersValue) {
		headersObj = headersValue.ToObject(rt)
	}

	if err := paramsObj.Set("headers", headersObj); err != nil {
		return args, err
	}

	for key, value := range traceContext {
		if err := headersObj.Set(key, value); err != nil {
			return args, fmt.Errorf("failed to set the trace header; reason: %w", err)
		}
	}

	return args, nil
}
