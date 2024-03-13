// Package sse implements a k6/sse for k6. It provides basic functionality to handle Server-Sent Event over http
// that *blocks* the event loop while the http connection is opened.
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
	ctx           context.Context //nolint:containedctx
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

// Open establishes a client connection based on the parameters provided.
//
//nolint:funlen
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

	//nolint:bodyclose // as it's deferred closed in closeResponseBody
	client, httpResponse, connEndHook, err := mi.open(ctx, state, rt, url, parsedArgs)
	defer connEndHook()
	if err != nil {
		// Pass the error to the user script before exiting immediately
		client.handleEvent("error", rt.ToValue(err))
		if state.Options.Throw.Bool {
			return nil, err
		}
		if httpResponse != nil {
			return wrapHTTPResponse(httpResponse)
		}
		return &HTTPResponse{Error: err.Error()}, nil
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

	reader := bufio.NewReader(httpResponse.Body)

	// Wraps a couple of channels
	go client.readEvents(reader, readEventChan, readErrChan, readCloseChan)

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
			sseResponse, sseRespErr := wrapHTTPResponse(httpResponse)
			if sseRespErr != nil {
				return nil, sseRespErr
			}
			sseResponse.URL = url
			return sseResponse, nil
		}
	}
}

func (mi *sse) open(
	ctx context.Context, state *lib.State, rt *goja.Runtime, url string,
	args *sseOpenArgs,
) (*Client, *http.Response, func(), error) {
	client := Client{
		ctx:            ctx,
		rt:             rt,
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
	// this is needed because of how interfaces work and that ssed.Jar is http.Cookiejar
	if args.cookieJar != nil {
		httpClient.Jar = args.cookieJar
	}

	httpMethod := http.MethodGet
	if args.method != "" {
		httpMethod = args.method
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, url, strings.NewReader(args.body))
	if err != nil {
		return &client, nil, nil, err
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if state.Options.SystemTags.Has(metrics.TagIP) {
				if ip, _, err2 := net.SplitHostPort(connInfo.Conn.RemoteAddr().String()); err2 == nil {
					args.tagsAndMeta.SetSystemTagOrMeta(metrics.TagIP, ip)
				}
			}
		},
	}

	//nolint:contextcheck // as it's passed in the request
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	connStart := time.Now()
	resp, err := httpClient.Do(req)
	connEnd := time.Now()

	if resp != nil {
		client.resp = resp
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			args.tagsAndMeta.SetSystemTagOrMeta(
				metrics.TagStatus, strconv.Itoa(resp.StatusCode))
		}
	}

	connEndHook := client.pushSessionMetrics(connStart, connEnd)

	return &client, resp, connEndHook, err
}

// On is used to configure what the client should do on each event.
func (s *Client) On(event string, handler goja.Value) {
	if handler, ok := goja.AssertFunction(handler); ok {
		s.eventHandlers[event] = append(s.eventHandlers[event], handler)
	}
}

func (s *Client) handleEvent(event string, args ...goja.Value) {
	if handlers, ok := s.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(goja.Undefined(), args...); err != nil {
				common.Throw(s.rt, err)
			}
		}
	}
}

// closeResponseBody cleanly closes the response body.
// Returns an error if sending the response body cannot be closed.
func (s *Client) closeResponseBody() error {
	var err error

	s.shutdownOnce.Do(func() {
		err = s.resp.Body.Close()
		if err != nil {
			// Call the user-defined error handler
			s.handleEvent("error", s.rt.ToValue(err))
		}
		close(s.done)
	})

	return err
}

func (s *Client) pushSessionMetrics(connStart, connEnd time.Time) func() {
	connDuration := metrics.D(connEnd.Sub(connStart))

	metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: s.builtinMetrics.HTTPReqSending,
					Tags:   s.tagsAndMeta.Tags,
				},
				Time:     connStart,
				Metadata: s.tagsAndMeta.Metadata,
				Value:    connDuration,
			},
		},
		Tags: s.tagsAndMeta.Tags,
		Time: connStart,
	})

	return func() {
		end := time.Now()
		requestDuration := metrics.D(end.Sub(connStart))

		metrics.PushIfNotDone(s.ctx, s.samplesOutput, metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					TimeSeries: metrics.TimeSeries{
						Metric: s.builtinMetrics.HTTPReqs,
						Tags:   s.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: s.tagsAndMeta.Metadata,
					Value:    1,
				},
				{
					TimeSeries: metrics.TimeSeries{
						Metric: s.builtinMetrics.HTTPReqSending,
						Tags:   s.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: s.tagsAndMeta.Metadata,
					Value:    connDuration,
				},
				{
					TimeSeries: metrics.TimeSeries{
						Metric: s.builtinMetrics.HTTPReqDuration,
						Tags:   s.tagsAndMeta.Tags,
					},
					Time:     end,
					Metadata: s.tagsAndMeta.Metadata,
					Value:    requestDuration,
				},
			},
			Tags: s.tagsAndMeta.Tags,
			Time: end,
		})
	}
}

// Wraps SSE in a channel
func (s *Client) readEvents(reader *bufio.Reader, readChan chan Event, errorChan chan error, closeChan chan int) {
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
				case <-s.done:
					return
				}
			} else {
				select {
				case errorChan <- err:
					return
				case <-s.done:
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
				case <-s.done:
					return
				}
			}
		default:
			select {
			case errorChan <- errors.New("unknown event: " + string(line)):
			case <-s.done:
				return
			}
		}
	}
}

// Wrap the raw HTTPResponse we received to a sseHTTPResponse we can pass to the user
func wrapHTTPResponse(httpResponse *http.Response) (*HTTPResponse, error) {
	sseResponse := HTTPResponse{
		Status: httpResponse.StatusCode,
	}

	sseResponse.Headers = make(map[string]string, len(httpResponse.Header))
	for k, vs := range httpResponse.Header {
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
