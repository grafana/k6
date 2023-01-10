package tracing

import (
	"errors"
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

type (
	// HTTPRequestFunc is a type alias representing the prototype of
	// k6's http module's request function
	HTTPRequestFunc func(method string, url goja.Value, args ...goja.Value) (*httpmodule.Response, error)
)

// NewClient instantiates a new tracing Client
func NewClient(vu modules.VU, opts options) *Client {
	rt := vu.Runtime()

	// Instantiate the http module in our runtime and get its default exported object
	httpModuleObj, ok := httpmodule.New().NewModuleInstance(vu).Exports().Default.(*goja.Object)
	if !ok {
		common.Throw(rt, errors.New("failed to initialize tracing client, unable to load http module"))
	}

	// Get the http module's request function
	httpModuleRequest := httpModuleObj.Get("request")

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
	case "b3":
		c.propagator = &B3Propagator{}
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

	// Ensure the arguments have a params object, in which
	// we can add the tracing headers at a later point in time.
	args, params, err := c.getOrCreateParams(method, args...)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to normalize the params argument; reason: %w", err))
	}

	// Ensure that the params object contains a headers object.
	// Create it if it doesn't.
	headers, err := c.getOrCreateHeaders(params)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to normalize params argument headers property; reason: %w", err))
	}

	traceID := NewTraceID(k6Prefix, k6CloudCode, uint64(time.Now().UnixNano()))
	encodedTraceID, _, err := traceID.Encode()
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to encode the generated trace ID; reason: %w", err))
	}

	// Produce a trace header in the format defined by the
	// configured propagator.
	header, err := c.propagator.Propagate(encodedTraceID)
	if err != nil {
		common.Throw(rt, fmt.Errorf("failed to propagate trace ID; reason: %w", err))
	}

	for key, value := range header {
		err = headers.Set(key, value)
		if err != nil {
			common.Throw(rt, fmt.Errorf("failed to set the trace header; reason: %w", err))
		}
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

// Delete instruments the http module's delete method.
func (c *Client) Delete(url goja.Value, args ...goja.Value) (*httpmodule.Response, error) {
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

// getOrCreateParams ensures that the HTTP method arguments list contains
// a params object. If it doesn't, it creates one.
//
// The method returns the normalized arguments list as well as its params
// object.
//
// Note that as of k6 v0.42.0 the HTTP API can turn out to be a bit inconsistent,
// as it can be called with 0, 1 or 2 arguments, and the second argument
// can be either a request's body, or a params object.
func (c *Client) getOrCreateParams(method string, args ...goja.Value) ([]goja.Value, *goja.Object, error) {
	rt := c.vu.Runtime()
	params := rt.NewObject()

	// The first argument of http methods is the `this` argument, so we need to shift
	// the arguments list by one. Thus, the cases below correspond to the effective
	// argument count of the http method + 1.
	switch len(args) {
	case 2, 3:
		// The second (or third) argument is the params object.
		paramsValue := args[len(args)-1]
		if !isNullish(paramsValue) {
			params = paramsValue.ToObject(rt)
			break
		}

		args[len(args)-1] = params
	case 1:
		// The http.get and the http.head methods take an optional params
		// object as first argument. Whereas the other methods take a
		// request's body as first argument, and an optional params object
		// as second argument.
		if method != http.MethodGet && method != http.MethodHead {
			args = append(args, params)
			break
		}

		// The first argument is the params object.
		paramsValue := args[0]
		if !isNullish(paramsValue) {
			params = paramsValue.ToObject(rt)
			break
		}

		args[0] = params
	case 0:
		// No arguments, we'll add a params object as the second argument.
		args = []goja.Value{goja.Null(), params}
	default:
		return args, params, fmt.Errorf("unexpected number of arguments for http.%s method", method)
	}

	return args, params, nil
}

// getOrCreateHeaders ensures that a http method params object is properly
// formed, and has the expected properties set.
//
// This method modifies the params object in place, and returns the headers object.
func (c *Client) getOrCreateHeaders(params *goja.Object) (*goja.Object, error) {
	rt := c.vu.Runtime()

	headersValue := params.Get("headers")
	if !isNullish(headersValue) {
		return headersValue.ToObject(rt), nil
	}

	headers := rt.NewObject()
	if err := params.Set("headers", headers); err != nil {
		return nil, err
	}

	return headers, nil
}
