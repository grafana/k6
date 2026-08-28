// Package sse implements a k6/x/sse javascript module extension for k6.
// It provides basic functionality to handle Server-Sent Event over http
// that *blocks* the event loop while the http connection is opened.
// [SSE API design document]:
// https://github.com/phymbert/xk6-sse/blob/master/docs/design/021-sse-api.md#proposed-solution
package sse

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6-extension-api"
	"go.k6.io/k6-extension-api/common"
)

type (
	// sse represents a module instance of the sse module.
	sse struct {
		vu      extensionapi.VU
		obj     *sobek.Object
		metrics *sseMetrics
	}
)

// ErrSSEInInitContext is returned when sse are using in the init context
var ErrSSEInInitContext = errors.New("using sse in the init context is not supported")

// Client is the representation of the sse returned to the js.
type Client struct {
	rt            *sobek.Runtime
	ctx           context.Context
	url           string
	resp          *http.Response
	eventHandlers map[string][]sobek.Callable
	done          chan struct{}
	shutdownOnce  sync.Once

	tags          extensionapi.Tags
	metrics       extensionapi.Metrics
	sseMetrics    *sseMetrics
	cancelRequest context.CancelFunc
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
	ID      string `js:"id"`
	Comment string
	Name    string
	Data    string
}

type sseOpenArgs struct {
	setupFn   sobek.Callable
	headers   http.Header
	method    string
	body      string
	cookieJar http.CookieJar
	tags      extensionapi.Tags
	timeout   time.Duration
}

// Exports returns the exports of the sse module.
func (mi *sse) Exports() extensionapi.Exports {
	return extensionapi.Exports{Default: mi.obj}
}

// Open establishes a http client connection based on the parameters provided.
func (mi *sse) Open(url string, args ...sobek.Value) (*HTTPResponse, error) {
	ctx := mi.vu.Context()
	rt := mi.vu.Runtime()
	if execution, ok := mi.vu.(extensionapi.Execution); ok && execution.ExecutionPhase() != extensionapi.ExecutionPhaseVU {
		return nil, ErrSSEInInitContext
	}
	metrics, ok := mi.vu.(extensionapi.Metrics)
	if !ok {
		return nil, extensionapi.ErrMetricsUnavailable
	}

	parsedArgs, err := parseConnectArgs(metrics, rt, args...)
	if err != nil {
		return nil, err
	}

	client, err := mi.open(ctx, rt, url, parsedArgs, metrics)
	if err != nil {
		// Pass the error to the user script before exiting immediately
		client.handleEvent("error", rt.ToValue(err))
		return nil, err
	}

	// Run the user-provided set up function
	if _, err := parsedArgs.setupFn(sobek.Undefined(), rt.ToValue(&client)); err != nil {
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
			_ = client.metrics.Emit(ctx, []extensionapi.Sample{{
				Metric: client.sseMetrics.SSEEventReceived,
				Value:  1,
				Time:   time.Now(),
				Tags:   client.tags,
			}})

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
			return client.wrapHTTPResponse(""), nil
		}
	}
}

func (mi *sse) open(
	ctx context.Context, rt *sobek.Runtime, url string, args *sseOpenArgs, metrics extensionapi.Metrics,
) (*Client, error) {
	var reqCtx context.Context
	var cancel context.CancelFunc
	if args.timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, args.timeout)
	} else {
		reqCtx, cancel = context.WithCancel(ctx)
	}

	sseClient := Client{
		ctx:           ctx,
		rt:            rt,
		url:           url,
		eventHandlers: make(map[string][]sobek.Callable),
		done:          make(chan struct{}),
		tags:          args.tags,
		metrics:       metrics,
		sseMetrics:    mi.metrics,
		cancelRequest: cancel,
	}

	httpMethod := http.MethodGet
	if args.method != "" {
		httpMethod = args.method
	}

	req, err := http.NewRequestWithContext(reqCtx, httpMethod, url, strings.NewReader(args.body))
	if err != nil {
		return &sseClient, err
	}

	req.Header.Set("Accept", "text/event-stream")
	for headerName, headerValues := range args.headers {
		for _, headerValue := range headerValues {
			req.Header.Set(headerName, headerValue)
		}
	}

	httpClient, ok := mi.vu.(extensionapi.HTTP)
	if !ok {
		return &sseClient, extensionapi.ErrHTTPUnavailable
	}

	//nolint:bodyclose // response body is closed by closeResponseBody.
	response, err := httpClient.Do(reqCtx, req, extensionapi.HTTPOptions{
		Jar:          args.cookieJar,
		Tags:         args.tags,
		ForceHTTP1:   true,
		DeferMetrics: true,
	})
	if response != nil {
		sseClient.resp = response.Response
		sseClient.tags = metrics.WithSystemTags(sseClient.tags, map[extensionapi.SystemTag]string{
			extensionapi.SystemTagURL:    url,
			extensionapi.SystemTagStatus: fmt.Sprintf("%d", response.Response.StatusCode),
			extensionapi.SystemTagProto:  response.Response.Proto,
		})
	}

	return &sseClient, err
}

// On is used to configure what the client should do on each event.
func (c *Client) On(event string, handler sobek.Value) {
	if handler, ok := sobek.AssertFunction(handler); ok {
		c.eventHandlers[event] = append(c.eventHandlers[event], handler)
	}
}

// Close the event loop
func (c *Client) Close() error {
	err := c.closeResponseBody()
	c.cancelRequest()
	return err
}

func (c *Client) handleEvent(event string, args ...sobek.Value) {
	if handlers, ok := c.eventHandlers[event]; ok {
		for _, handler := range handlers {
			if _, err := handler(sobek.Undefined(), args...); err != nil {
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
		if c.resp != nil {
			err = c.resp.Body.Close()
		}
		if err != nil {
			c.handleEvent("error", c.rt.ToValue(err))
		}
		close(c.done)
	})

	return err
}

// Wraps SSE in a channel, follow the SSE format described in:
// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events
func (c *Client) readEvents(readChan chan Event, errorChan chan error, closeChan chan int) {
	reader := bufio.NewReader(c.resp.Body)
	ev := Event{}
	var buf bytes.Buffer

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
		// id of event
		case hasPrefix(line, "id: "):
			ev.ID = stripPrefix(line, 4)
		case hasPrefix(line, "id:"):
			ev.ID = stripPrefix(line, 3)

		// Comment
		case hasPrefix(line, ": "):
			ev.Comment = stripPrefix(line, 2)
		case hasPrefix(line, ":"):
			ev.Comment = stripPrefix(line, 1)

		// name of event
		case hasPrefix(line, "event: "):
			ev.Name = stripPrefix(line, 7)
		case hasPrefix(line, "event:"):
			ev.Name = stripPrefix(line, 6)

		// event data
		case hasPrefix(line, "data: "):
			buf.Write(line[6:])

		case hasPrefix(line, "data:"):
			buf.Write(line[5:])

		case hasPrefix(line, "retry:"):
			// Retry, do nothing for now

		// end of event
		case isLineEnd(line):
			// Trailing newlines are removed.
			ev.Data = strings.TrimRightFunc(buf.String(), func(r rune) bool {
				return r == '\r' || r == '\n'
			})

			select {
			case readChan <- ev:
				buf.Reset()
				ev = Event{}
			case <-c.done:
				return
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

func isLineEnd(line []byte) bool {
	return bytes.Equal(line, []byte("\n")) || bytes.Equal(line, []byte("\r\n"))
}

// Wrap the raw HTTPResponse we received to a sse.HTTPResponse we can pass to the user
func (c *Client) wrapHTTPResponse(errMessage string) *HTTPResponse {
	if errMessage != "" {
		return &HTTPResponse{Error: errMessage}
	}
	sseResponse := HTTPResponse{
		URL:    c.url,
		Status: c.resp.StatusCode,
	}

	sseResponse.Headers = make(map[string]string, len(c.resp.Header))
	for k, vs := range c.resp.Header {
		sseResponse.Headers[k] = strings.Join(vs, ", ")
	}

	return &sseResponse
}

func parseConnectArgs(metrics extensionapi.Metrics, rt *sobek.Runtime, args ...sobek.Value) (*sseOpenArgs, error) {
	// The params argument is optional
	var callableV, paramsV sobek.Value
	switch len(args) {
	case 2:
		paramsV = args[0]
		callableV = args[1]
	case 1:
		paramsV = sobek.Undefined()
		callableV = args[0]
	default:
		return nil, errors.New("invalid number of arguments to sse.open")
	}
	// Get the callable (required)
	setupFn, isFunc := sobek.AssertFunction(callableV)
	if !isFunc {
		return nil, errors.New("last argument to sse.open must be a function")
	}

	headers := make(http.Header)
	parsedArgs := &sseOpenArgs{
		setupFn: setupFn,
		headers: headers,
		tags:    metrics.WithSystemTags(metrics.CurrentTags(), map[extensionapi.SystemTag]string{extensionapi.SystemTagSubproto: ""}),
		timeout: 0,
	}

	if sobek.IsUndefined(paramsV) || sobek.IsNull(paramsV) {
		return parsedArgs, nil
	}

	err := parseConnectOptionalArgs(paramsV, rt, parsedArgs)
	if err != nil {
		return nil, err
	}

	return parsedArgs, nil
}

func parseConnectOptionalArgs(paramsV sobek.Value, rt *sobek.Runtime, parsedArgs *sseOpenArgs) error {
	params := paramsV.ToObject(rt)
	for _, k := range params.Keys() {
		switch k {
		case "headers":
			headersV := params.Get(k)
			if sobek.IsUndefined(headersV) || sobek.IsNull(headersV) {
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
			tagsValue := params.Get(k)
			if sobek.IsUndefined(tagsValue) || sobek.IsNull(tagsValue) {
				continue
			}
			tagsObject := tagsValue.ToObject(rt)
			if tagsObject == nil {
				continue
			}
			tags := make(map[string]string, len(tagsObject.Keys()))
			for _, key := range tagsObject.Keys() {
				tags[key] = tagsObject.Get(key).String()
			}
			parsedArgs.tags = parsedArgs.tags.With(tags)
		case "jar":
			jarV := params.Get(k)
			if sobek.IsUndefined(jarV) || sobek.IsNull(jarV) {
				continue
			}
			if jar, ok := exportedCookieJar(jarV.Export()); ok {
				parsedArgs.cookieJar = jar
			}
		case "method":
			parsedArgs.method = strings.TrimSpace(params.Get(k).ToString().String())
		case "body":
			parsedArgs.body = strings.TrimSpace(params.Get(k).ToString().String())
		case "timeout":
			timeoutV := params.Get(k)
			if sobek.IsUndefined(timeoutV) || sobek.IsNull(timeoutV) {
				continue
			}
			timeout, err := time.ParseDuration(timeoutV.ToString().String())
			if err != nil {
				return fmt.Errorf("invalid sse.open() timeout: %w", err)
			}
			parsedArgs.timeout = timeout
		}
	}
	return nil
}

func exportedCookieJar(value any) (http.CookieJar, bool) {
	if jar, ok := value.(http.CookieJar); ok {
		return jar, true
	}

	// Older k6/http CookieJar values predate its implementation of
	// net/http.CookieJar but expose the same Jar field. This preserves custom
	// jars across supported host versions without importing a k6 package.
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Ptr {
		if reflected.IsNil() {
			return nil, false
		}
		reflected = reflected.Elem()
	}
	if reflected.Kind() != reflect.Struct {
		return nil, false
	}
	field := reflected.FieldByName("Jar")
	if !field.IsValid() || !field.CanInterface() {
		return nil, false
	}
	jar, ok := field.Interface().(http.CookieJar)
	return jar, ok
}

func hasPrefix(s []byte, prefix string) bool {
	return bytes.HasPrefix(s, []byte(prefix))
}

func stripPrefix(line []byte, start int) string {
	return string(line[start : len(line)-1])
}
