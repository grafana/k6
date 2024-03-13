// Package sse implements a k6/sse javascript module extension for k6.
// It provides basic functionality to handle Server-Sent Event over http
// that *blocks* the event loop while the http connection is opened.
// [File API design document]: https://github.com/grafana/k6/blob/master/docs/design/021-sse-api.md#proposed-solution
package sse

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	httpModule "go.k6.io/k6/js/modules/k6/http"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// sse represents a module instance of the sse module.
	sse struct {
		vu  modules.VU
		obj *goja.Object
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &sse{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(m modules.VU) modules.Instance {
	rt := m.Runtime()
	mi := &sse{
		vu: m,
	}
	obj := rt.NewObject()
	if err := obj.Set("open", mi.Open); err != nil {
		common.Throw(rt, err)
	}

	mi.obj = obj
	return mi
}

// ErrSSEInInitContext is returned when sse are using in the init context
var ErrSSEInInitContext = common.NewInitContextError("using sse in the init context is not supported")

// Client is the representation of the sse returned to the js.
type Client struct {
	rt            *goja.Runtime
	ctx           context.Context
	url           string
	resp          *http.Response
	eventHandlers map[string][]goja.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	tagsAndMeta    *metrics.TagsAndMeta
	samplesOutput  chan<- metrics.SampleContainer
	builtinMetrics *metrics.BuiltinMetrics
}

// HTTPResponse is the http response returned by sse.open.
type HTTPResponse struct {
	URL     string            `json:"url"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Error   string            `json:"error"`
}

// Event represents a Server-Sent Event
type Event struct {
	ID   string
	Name string
	Data string
}

type sseOpenArgs struct {
	setupFn     goja.Callable
	headers     http.Header
	method      string
	body        string
	cookieJar   *cookiejar.Jar
	tagsAndMeta *metrics.TagsAndMeta
}

// Exports returns the exports of the sse module.
func (mi *sse) Exports() modules.Exports {
	return modules.Exports{Default: mi.obj}
}

// Open establishes a http client connection based on the parameters provided.
func (mi *sse) Open(url string, args ...goja.Value) (*HTTPResponse, error) {
	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	state := mi.vu.State()
	if state == nil {
		return nil, ErrSSEInInitContext
	}

	parsedArgs, err := parseConnectArgs(state, rt, args...)
	if err != nil {
		return nil, err
	}

	parsedArgs.tagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagURL, url)

	client, connEndHook, err := mi.open(ctx, state, rt, url, parsedArgs)
	defer connEndHook()
	if err != nil {
		// Pass the error to the user script before exiting immediately
		client.handleEvent("error", rt.ToValue(err))
		if state.Options.Throw.Bool {
			return nil, err
		}
		return client.wrapHTTPResponse(err.Error())
	}

	// Run the user-provided set up function
	if _, err := parsedArgs.setupFn(goja.Undefined(), rt.ToValue(&client)); err != nil {
		_ = client.closeResponseBody()
		return nil, err
	}

	// The connection is now open, emit the event
	client.handleEvent("open")

	readEventChan := make(chan Event)
	readErrChan := make(chan error)
	readCloseChan := make(chan int)

	// Wraps a couple of channels
	go client.readEvents(readEventChan, readErrChan, readCloseChan)

	// This is the main control loop. All JS code (including error handlers)
	// should only be executed by this thread to avoid race conditions
	for {
		select {
		case event := <-readEventChan:
			metrics.PushIfNotDone(ctx, client.samplesOutput, metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: client.builtinMetrics.SSEEventReceived,
					Tags:   client.tagsAndMeta.Tags,
				},
				Time:     time.Now(),
				Metadata: client.tagsAndMeta.Metadata,
				Value:    1,
			})

			client.handleEvent("event", rt.ToValue(event))

		case readErr := <-readErrChan:
			client.handleEvent("error", rt.ToValue(readErr))

		case <-ctx.Done():
			// VU is shutting down during an interrupt
			// client events will not be forwarded to the VU
			_ = client.closeResponseBody()

		case <-readCloseChan:
			_ = client.closeResponseBody()

		case <-client.done:
			// This is the final exit point normally triggered by closeResponseBody
			return client.wrapHTTPResponse("")
		}
	}
}

func (mi *sse) open(ctx context.Context, state *lib.State,
	rt *goja.Runtime, url string, args *sseOpenArgs,
) (*Client, func(), error) {
	sseClient := Client{
		ctx:            ctx,
		rt:             rt,
		url:            url,
		eventHandlers:  make(map[string][]goja.Callable),
		done:           make(chan struct{}),
		samplesOutput:  state.Samples,
		tagsAndMeta:    args.tagsAndMeta,
		builtinMetrics: state.BuiltinMetrics,
	}

	// Overriding the NextProtos to avoid talking http2
	var tlsConfig *tls.Config
	if state.TLSConfig != nil {
		tlsConfig = state.TLSConfig.Clone()
		tlsConfig.NextProtos = []string{"http/1.1"}
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	// httpClient.Jar must never be nil
	if args.cookieJar != nil {
		httpClient.Jar = args.cookieJar
	}

	httpMethod := http.MethodGet
	if args.method != "" {
		httpMethod = args.method
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, url, strings.NewReader(args.body))
	if err != nil {
		return &sseClient, nil, err
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	// Wrap the request to retrieve the server IP tag
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if state.Options.SystemTags.Has(metrics.TagIP) {
				if ip, _, err2 := net.SplitHostPort(connInfo.Conn.RemoteAddr().String()); err2 == nil {
					args.tagsAndMeta.SetSystemTagOrMeta(metrics.TagIP, ip)
				}
			}
		},
	}

	//nolint:contextcheck // parent context already passed in the request
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	connStart := time.Now()
	//nolint:bodyclose // Body is deferred closed in closeResponseBody
	resp, err := httpClient.Do(req)
	connEnd := time.Now()

	if resp != nil {
		sseClient.resp = resp
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			args.tagsAndMeta.SetSystemTagOrMeta(
				metrics.TagStatus, strconv.Itoa(resp.StatusCode))
		}
	}

	connEndHook := sseClient.pushSSEMetrics(connStart, connEnd)

	return &sseClient, connEndHook, err
}

// On is used to configure what the client should do on each event.
func (c *Client) On(event string, handler goja.Value) {
	if handler, ok := goja.AssertFunction(handler); ok {
		c.eventHandlers[event] = append(c.eventHandlers[event], handler)
	}
}

func (c *Client) handleEvent(event string, args ...goja.Value) {
	if handlers, ok := c.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(goja.Undefined(), args...); err != nil {
				common.Throw(c.rt, err)
			}
		}
	}
}

// closeResponseBody cleanly closes the response body.
// Returns an error if sending the response body cannot be closed.
func (c *Client) closeResponseBody() error {
	var err error

	c.shutdownOnce.Do(func() {
		err = c.resp.Body.Close()
		if err != nil {
			// Call the user-defined error handler
			c.handleEvent("error", c.rt.ToValue(err))
		}
		close(c.done)
	})

	return err
}

func (c *Client) pushSSEMetrics(connStart, connEnd time.Time) func() {
	connDuration := metrics.D(connEnd.Sub(connStart))

	metrics.PushIfNotDone(c.ctx, c.samplesOutput, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.builtinMetrics.HTTPReqSending,
					Tags:   c.tagsAndMeta.Tags,
				},
				Time:     connStart,
				Metadata: c.tagsAndMeta.Metadata,
				Value:    connDuration,
			},
		},
		Tags: c.tagsAndMeta.Tags,
		Time: connStart,
	})

	return func() {
		end := time.Now()
		requestDuration := metrics.D(end.Sub(connStart))

		metrics.PushIfNotDone(c.ctx, c.samplesOutput, metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					TimeSeries: metrics.TimeSeries{
						Metric: c.builtinMetrics.HTTPReqs,
						Tags:   c.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: c.tagsAndMeta.Metadata,
					Value:    1,
				},
				{
					TimeSeries: metrics.TimeSeries{
						Metric: c.builtinMetrics.HTTPReqSending,
						Tags:   c.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: c.tagsAndMeta.Metadata,
					Value:    connDuration,
				},
				{
					TimeSeries: metrics.TimeSeries{
						Metric: c.builtinMetrics.HTTPReqDuration,
						Tags:   c.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: c.tagsAndMeta.Metadata,
					Value:    requestDuration,
				},
			},
			Tags: c.tagsAndMeta.Tags,
			Time: end,
		})
	}
}

// Wraps SSE in a channel
func (c *Client) readEvents(readChan chan Event, errorChan chan error, closeChan chan int) {
	reader := bufio.NewReader(c.resp.Body)
	ev := Event{}
	var buf bytes.Buffer

	sendEvent := false
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				select {
				case closeChan <- -1:
					return
				case <-c.done:
					return
				}
			} else {
				select {
				case errorChan <- err:
					return
				case <-c.done:
					return
				}
			}
		}

		switch {
		case hasPrefix(line, ":"):
			// Comment, do nothing

		case hasPrefix(line, "retry:"):
			// Retry, do nothing for now

		// id of event
		case hasPrefix(line, "id: "):
			ev.ID = stripPrefix(line, 4)
		case hasPrefix(line, "id:"):
			ev.ID = stripPrefix(line, 3)

		// name of event
		case hasPrefix(line, "event: "):
			ev.Name = stripPrefix(line, 7)
		case hasPrefix(line, "event:"):
			ev.Name = stripPrefix(line, 6)

		// event data
		case hasPrefix(line, "data: "):
			buf.Write(line[6:])
			sendEvent = true
		case hasPrefix(line, "data:"):
			buf.Write(line[5:])
			sendEvent = true

		// end of event
		case bytes.Equal(line, []byte("\n")):
			if sendEvent {
				// Report an unexpected closure
				ev.Data = buf.String()
				select {
				case readChan <- ev:
					sendEvent = false
					buf.Reset()
					ev = Event{}
				case <-c.done:
					return
				}
			}
		default:
			select {
			case errorChan <- errors.New("unknown event: " + string(line)):
			case <-c.done:
				return
			}
		}
	}
}

// Wrap the raw HTTPResponse we received to a sse.HTTPResponse we can pass to the user
func (c *Client) wrapHTTPResponse(errMessage string) (*HTTPResponse, error) {
	if errMessage != "" {
		return &HTTPResponse{Error: errMessage}, nil
	}
	sseResponse := HTTPResponse{
		URL:    c.url,
		Status: c.resp.StatusCode,
	}

	sseResponse.Headers = make(map[string]string, len(c.resp.Header))
	for k, vs := range c.resp.Header {
		sseResponse.Headers[k] = strings.Join(vs, ", ")
	}

	return &sseResponse, nil
}

func parseConnectArgs(state *lib.State, rt *goja.Runtime, args ...goja.Value) (*sseOpenArgs, error) {
	// The params argument is optional
	var callableV, paramsV goja.Value
	switch len(args) {
	case 2:
		paramsV = args[0]
		callableV = args[1]
	case 1:
		paramsV = goja.Undefined()
		callableV = args[0]
	default:
		return nil, errors.New("invalid number of arguments to sse.open")
	}
	// Get the callable (required)
	setupFn, isFunc := goja.AssertFunction(callableV)
	if !isFunc {
		return nil, errors.New("last argument to sse.open must be a function")
	}

	headers := make(http.Header)
	headers.Set("User-Agent", state.Options.UserAgent.String)
	tagsAndMeta := state.Tags.GetCurrentValues()
	parsedArgs := &sseOpenArgs{
		setupFn:     setupFn,
		headers:     headers,
		cookieJar:   state.CookieJar,
		tagsAndMeta: &tagsAndMeta,
	}

	if goja.IsUndefined(paramsV) || goja.IsNull(paramsV) {
		return parsedArgs, nil
	}

	// Parse the optional second argument (params)
	params := paramsV.ToObject(rt)
	for _, k := range params.Keys() {
		switch k {
		case "headers":
			headersV := params.Get(k)
			if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
				continue
			}
			headersObj := headersV.ToObject(rt)
			if headersObj == nil {
				continue
			}
			for _, key := range headersObj.Keys() {
				parsedArgs.headers.Set(key, headersObj.Get(key).String())
			}
		case "tags":
			if err := common.ApplyCustomUserTags(rt, parsedArgs.tagsAndMeta, params.Get(k)); err != nil {
				return nil, fmt.Errorf("invalid sse.open() metric tags: %w", err)
			}
		case "jar":
			jarV := params.Get(k)
			if goja.IsUndefined(jarV) || goja.IsNull(jarV) {
				continue
			}
			if v, ok := jarV.Export().(*httpModule.CookieJar); ok {
				parsedArgs.cookieJar = v.Jar
			}
		case "method":
			parsedArgs.method = strings.TrimSpace(params.Get(k).ToString().String())
		case "body":
			parsedArgs.body = strings.TrimSpace(params.Get(k).ToString().String())
		}
	}

	return parsedArgs, nil
}

func hasPrefix(s []byte, prefix string) bool {
	return bytes.HasPrefix(s, []byte(prefix))
}

func stripPrefix(line []byte, start int) string {
	return string(line[start : len(line)-1])
}
