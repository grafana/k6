package tracing

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dop251/goja"
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
}

// HTTPRequestFunc is a type alias representing the prototype of
// k6's http module's request function
type HTTPRequestFunc func(method string, url goja.Value, args ...goja.Value) (*httpmodule.Response, error)

// NewClient instantiates a new tracing Client
func NewClient(vu modules.VU, opts options) *Client {
	rt := vu.Runtime()

	// Get the http module's request function
	httpModuleRequest, err := rt.RunString("require('k6/http').request")
	if err != nil {
		common.Throw(
			rt,
			fmt.Errorf(
				"failed initializing tracing client, "+
					"unable to require http.request method; reason: %w",
				err,
			),
		)
	}

	// Export the http module's request function goja.Callable as a Go function
	var requestFunc HTTPRequestFunc
	if err := rt.ExportTo(httpModuleRequest, &requestFunc); err != nil {
		common.Throw(
			rt,
			fmt.Errorf("failed initializing tracing client, unable to export http.request method; reason: %w", err),
		)
	}

	client := &Client{vu: vu, requestFunc: requestFunc}
	if err := client.Configure(opts); err != nil {
		common.Throw(
			rt,
			fmt.Errorf("failed initializing tracing client, invalid configuration; reason: %w", err),
		)
	}

	return client
}

// Configure configures the tracing client with the given options.
func (c *Client) Configure(opts options) error {
	if err := opts.validate(); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	switch opts.Propagator {
	case "w3c":
		c.propagator = &W3CPropagator{}
	case "jaeger":
		c.propagator = &JaegerPropagator{}
	default:
		return fmt.Errorf("unknown propagator: %s", opts.Propagator)
	}

	c.opts = opts

	return nil
}

// Request instruments the http module's request function with tracing headers,
// and ensures the trace_id is emitted as part of the output's data points metadata.
func (c *Client) Request(method string, url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	rt := c.vu.Runtime()

	// The http module's request function expects the first argument to be the
	// request body. If no body is provided, we need to pass null to the function.
	if len(args) == 0 {
		args = []goja.Value{goja.Null()}
	}

	traceID := TraceID{
		Prefix: k6Prefix,
		Code:   k6CloudCode,
		Time:   time.Now(),
	}

	encodedTraceID, err := traceID.Encode()
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to encode the generated trace ID; reason: %w", err))
	}

	// Produce a trace header in the format defined by the
	// configured propagator.
	traceContextHeader, err := c.propagator.Propagate(encodedTraceID)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to propagate trace ID; reason: %w", err))
	}

	// update the `params` argument with the trace context header
	// so that it can be used by the http module's request function.
	args, err = c.instrumentArguments(traceContextHeader, args...)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to instrument request arguments; reason: %w", err))
	}

	// Add the trace ID to the VU's state, so that it can be
	// used in the metrics emitted by the HTTP module.
	c.vu.State().Tags.Modify(func(t *metrics.TagsAndMeta) {
		t.SetMetadata(metadataTraceIDKeyName, encodedTraceID)
	})

	response, err := c.requestFunc(method, url, args...)
	if err != nil {
		common.Throw(rt, err)
	}

	// Remove the trace ID from the VU's state, so that it doesn't
	// leak into other requests.
	c.vu.State().Tags.Modify(func(t *metrics.TagsAndMeta) {
		t.DeleteMetadata(metadataTraceIDKeyName)
	})

	return response, nil
}

// Del instruments the http module's delete method.
func (c *Client) Del(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodDelete, url, args...)
}

// Get instruments the http module's get method.
func (c *Client) Get(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	// Here we prepend a null value that stands for the body parameter,
	// that the request function expects as a first argument implicitly
	args = append([]goja.Value{goja.Null()}, args...)
	return c.Request(http.MethodGet, url, args...)
}

// Head instruments the http module's head method.
func (c *Client) Head(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	// NB: here we prepend a null value that stands for the body parameter,
	// that the request function expects as a first argument implicitly
	args = append([]goja.Value{goja.Null()}, args...)
	return c.Request(http.MethodHead, url, args...)
}

// Options instruments the http module's options method.
func (c *Client) Options(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodOptions, url, args...)
}

// Patch instruments the http module's patch method.
func (c *Client) Patch(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPatch, url, args...)
}

// Post instruments the http module's post method.
func (c *Client) Post(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPost, url, args...)
}

// Put instruments the http module's put method.
func (c *Client) Put(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
	return c.Request(http.MethodPut, url, args...)
}

// instrumentArguments: expects args to be in the format expected by the
// request method (body, params)
func (c *Client) instrumentArguments(traceContext http.Header, args ...goja.Value) ([]goja.Value, error) {
	rt := c.vu.Runtime()

	var paramsObj *goja.Object

	switch len(args) {
	case 2:
		// We received both a body and a params argument. In the
		// event params would be nullish, we'll instantiate
		// a new object.
		if isNullish(args[1]) {
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
	if !isNullish(headersValue) {
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
