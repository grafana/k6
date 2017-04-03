package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptrace"
	neturl "net/url"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js2/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

type HTTPResponseTimings struct {
	Duration, Blocked, LookingUp, Connecting, Sending, Waiting, Receiving float64
}

type HTTPResponse struct {
	URL     string
	Status  int
	Headers map[string]string
	Body    string
	Timings HTTPResponseTimings

	runtime    *goja.Runtime
	cachedJSON goja.Value
}

func (res *HTTPResponse) Json() goja.Value {
	if res.cachedJSON == nil {
		var v interface{}
		if err := json.Unmarshal([]byte(res.Body), &v); err != nil {
			common.Throw(res.runtime, err)
		}
		res.cachedJSON = res.runtime.ToValue(v)
	}
	return res.cachedJSON
}

type HTTP struct{}

func (*HTTP) Request(ctx context.Context, method, url string, args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(ctx)
	state := common.GetState(ctx)

	var bodyReader io.Reader
	var contentType string
	if len(args) > 0 && !goja.IsUndefined(args[0]) && !goja.IsNull(args[0]) {
		var data map[string]goja.Value
		if rt.ExportTo(args[0], &data) == nil {
			bodyQuery := make(neturl.Values, len(data))
			for k, v := range data {
				bodyQuery.Set(k, v.String())
			}
			bodyReader = bytes.NewBufferString(bodyQuery.Encode())
			contentType = "application/x-www-form-urlencoded"
		} else {
			bodyReader = bytes.NewBufferString(args[0].String())
		}
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	tags := map[string]string{
		"status": "0",
		"method": method,
		"url":    url,
		"group":  state.Group.Path,
	}

	if len(args) > 1 {
		paramsV := args[1]
		if !goja.IsUndefined(paramsV) && !goja.IsNull(paramsV) {
			params := paramsV.ToObject(rt)
			for _, k := range params.Keys() {
				switch k {
				case "headers":
					headersV := params.Get(k)
					if goja.IsUndefined(headersV) || goja.IsNull(headersV) {
						continue
					}
					headers := headersV.ToObject(rt)
					if headers == nil {
						continue
					}
					for _, key := range headers.Keys() {
						req.Header.Set(key, headers.Get(key).String())
					}
				case "tags":
					tagsV := params.Get(k)
					if goja.IsUndefined(tagsV) || goja.IsNull(tagsV) {
						continue
					}
					tagObj := tagsV.ToObject(rt)
					if tagObj == nil {
						continue
					}
					for _, key := range tagObj.Keys() {
						tags[key] = tagObj.Get(key).String()
					}
				}
			}
		}
	}

	client := http.Client{}
	tracer := lib.Tracer{}
	res, err := client.Do(req.WithContext(httptrace.WithClientTrace(ctx, tracer.Trace())))
	if err != nil {
		state.Samples = append(state.Samples, tracer.Done().Samples(tags)...)
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		state.Samples = append(state.Samples, tracer.Done().Samples(tags)...)
		return nil, err
	}
	_ = res.Body.Close()
	trail := tracer.Done()

	tags["status"] = strconv.Itoa(res.StatusCode)
	state.Samples = append(state.Samples, trail.Samples(tags)...)

	headers := make(map[string]string, len(res.Header))
	for k, vs := range res.Header {
		headers[k] = strings.Join(vs, ", ")
	}
	return &HTTPResponse{
		URL:     res.Request.URL.String(),
		Status:  res.StatusCode,
		Headers: headers,
		Body:    string(body),
		Timings: HTTPResponseTimings{
			Duration:   stats.D(trail.Duration),
			Blocked:    stats.D(trail.Blocked),
			LookingUp:  stats.D(trail.LookingUp),
			Connecting: stats.D(trail.Connecting),
			Sending:    stats.D(trail.Sending),
			Waiting:    stats.D(trail.Waiting),
			Receiving:  stats.D(trail.Receiving),
		},

		runtime: rt,
	}, nil
}

func (http *HTTP) Get(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "GET", url, args...)
}

func (http *HTTP) Head(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	// The body argument is always undefined for GETs and HEADs.
	args = append([]goja.Value{goja.Undefined()}, args...)
	return http.Request(ctx, "HEAD", url, args...)
}

func (http *HTTP) Post(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "POST", url, args...)
}

func (http *HTTP) Put(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PUT", url, args...)
}

func (http *HTTP) Patch(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "PATCH", url, args...)
}

func (http *HTTP) Del(ctx context.Context, url string, args ...goja.Value) (*HTTPResponse, error) {
	return http.Request(ctx, "DELETE", url, args...)
}
