package http

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/lib/types"
)

// ErrHTTPForbiddenInInitContext is used when a http requests was made in the init context
var ErrHTTPForbiddenInInitContext = common.NewInitContextError(
	"Making http requests in the init context is not supported")

// ErrBatchForbiddenInInitContext is used when batch was made in the init context
var ErrBatchForbiddenInInitContext = common.NewInitContextError(
	"Using batch in the init context is not supported")

func (c *Client) getMethodClosure(method string) func(url sobek.Value, args ...sobek.Value) (*Response, error) {
	return func(url sobek.Value, args ...sobek.Value) (*Response, error) {
		return c.Request(method, url, args...)
	}
}

// Request makes an http request of the provided `method` and returns a corresponding response by
// taking sobek.Values as arguments
func (c *Client) Request(method string, url sobek.Value, args ...sobek.Value) (*Response, error) {
	state := c.moduleInstance.vu.State()
	if state == nil {
		return nil, ErrHTTPForbiddenInInitContext
	}
	body, params := splitRequestArgs(args)

	req, err := c.parseRequest(method, url, body, params)
	if err != nil {
		return c.handleParseRequestError(err)
	}

	resp, err := httpext.MakeRequest(c.moduleInstance.vu.Context(), state, req)
	if err != nil {
		return nil, err
	}
	c.processResponse(resp, req.ResponseType)
	return c.responseFromHTTPext(resp), nil
}

func splitRequestArgs(args []sobek.Value) (body interface{}, params sobek.Value) {
	if len(args) > 0 {
		body = args[0].Export()
	}
	if len(args) > 1 {
		params = args[1]
	}
	return body, params
}

func (c *Client) handleParseRequestError(err error) (*Response, error) {
	state := c.moduleInstance.vu.State()

	if state.Options.Throw.Bool {
		return nil, err
	}
	state.Logger.WithField("error", err).Warn("Request Failed")
	r := httpext.NewResponse()
	r.Error = err.Error()
	var k6e httpext.K6Error
	if errors.As(err, &k6e) {
		r.ErrorCode = int(k6e.Code)
	}
	return &Response{Response: r, client: c}, nil
}

// asyncRequest makes an http request of the provided `method` and returns a promise. All the networking is done off
// the event loop and the returned promise will be resolved with the response or rejected with an error
func (c *Client) asyncRequest(method string, url sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
	state := c.moduleInstance.vu.State()
	if c.moduleInstance.vu.State() == nil {
		return nil, ErrHTTPForbiddenInInitContext
	}

	body, params := splitRequestArgs(args)
	rt := c.moduleInstance.vu.Runtime()
	req, err := c.parseRequest(method, url, body, params)
	p, resolve, reject := rt.NewPromise()
	if err != nil {
		var resp *Response
		if resp, err = c.handleParseRequestError(err); err != nil {
			err = reject(err)
		} else {
			err = resolve(resp)
		}
		return p, err
	}

	callback := c.moduleInstance.vu.RegisterCallback()

	go func() {
		resp, err := httpext.MakeRequest(c.moduleInstance.vu.Context(), state, req)
		callback(func() error {
			if err != nil {
				return reject(err)
			}
			c.processResponse(resp, req.ResponseType)
			return resolve(c.responseFromHTTPext(resp))
		})
	}()

	return p, nil
}

// processResponse stores the body as an ArrayBuffer if indicated by
// respType. This is done here instead of in httpext.readResponseBody to avoid
// a reverse dependency on js/common or sobek.
func (c *Client) processResponse(resp *httpext.Response, respType httpext.ResponseType) {
	if respType == httpext.ResponseTypeBinary && resp.Body != nil {
		b, ok := resp.Body.([]byte)
		if !ok {
			panic("got an unexpected type for the response body, only []byte is accepted")
		}
		resp.Body = c.moduleInstance.vu.Runtime().NewArrayBuffer(b)
	}
}

func (c *Client) responseFromHTTPext(resp *httpext.Response) *Response {
	return &Response{Response: resp, client: c}
}

// TODO: break this function up
//
//nolint:gocyclo, cyclop, funlen, gocognit
func (c *Client) parseRequest(
	method string, reqURL, body interface{}, params sobek.Value,
) (*httpext.ParsedHTTPRequest, error) {
	rt := c.moduleInstance.vu.Runtime()
	state := c.moduleInstance.vu.State()
	if state == nil {
		return nil, ErrHTTPForbiddenInInitContext
	}

	if urlJSValue, ok := reqURL.(sobek.Value); ok {
		reqURL = urlJSValue.Export()
	}
	u, err := httpext.ToURL(reqURL)
	if err != nil {
		return nil, err
	}

	result := &httpext.ParsedHTTPRequest{
		URL: &u,
		Req: &http.Request{
			Method: method,
			URL:    u.GetURL(),
			Header: make(http.Header),
		},
		Timeout:          60 * time.Second,
		Throw:            state.Options.Throw.Bool,
		Redirects:        state.Options.MaxRedirects,
		Cookies:          make(map[string]*httpext.HTTPRequestCookie),
		ResponseCallback: c.responseCallback,
		TagsAndMeta:      c.moduleInstance.vu.State().Tags.GetCurrentValues(),
	}

	if state.Options.DiscardResponseBodies.Bool {
		result.ResponseType = httpext.ResponseTypeNone
	} else {
		result.ResponseType = httpext.ResponseTypeText
	}

	formatFormVal := func(v interface{}) string {
		// TODO: handle/warn about unsupported/nested values
		return fmt.Sprintf("%v", v)
	}

	handleObjectBody := func(data map[string]interface{}) error {
		if !requestContainsFile(data) {
			bodyQuery := make(url.Values, len(data))
			for k, v := range data {
				if arr, ok := v.([]interface{}); ok {
					for _, el := range arr {
						bodyQuery.Add(k, formatFormVal(el))
					}
					continue
				}
				bodyQuery.Set(k, formatFormVal(v))
			}
			result.Body = bytes.NewBufferString(bodyQuery.Encode())
			result.Req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return nil
		}

		// handling multipart request
		result.Body = &bytes.Buffer{}
		mpw := multipart.NewWriter(result.Body)

		// For parameters of type common.FileData, created with open(file, "b"),
		// we write the file boundary to the body buffer.
		// Otherwise parameters are treated as standard form field.
		for k, v := range data {
			switch ve := v.(type) {
			case *FileData:
				// writing our own part to handle receiving
				// different content-type than the default application/octet-stream
				h := make(textproto.MIMEHeader)
				escapedFilename := escapeQuotes(ve.Filename)
				h.Set("Content-Disposition",
					fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
						k, escapedFilename))
				h.Set("Content-Type", ve.ContentType)

				// this writer will be closed either by the next part or
				// the call to mpw.Close()
				fw, err := mpw.CreatePart(h)
				if err != nil {
					return err
				}

				data, err := common.ToBytes(ve.Data)
				if err != nil {
					return err
				}
				if _, err := fw.Write(data); err != nil {
					return err
				}
			default:
				fw, err := mpw.CreateFormField(k)
				if err != nil {
					return err
				}

				if _, err := fw.Write([]byte(formatFormVal(v))); err != nil {
					return err
				}
			}
		}

		if err := mpw.Close(); err != nil {
			return err
		}

		result.Req.Header.Set("Content-Type", mpw.FormDataContentType())
		return nil
	}

	if body != nil {
		switch data := body.(type) {
		case map[string]sobek.Value:
			// TODO: fix forms submission and serialization in k6/html before fixing this..
			newData := map[string]interface{}{}
			for k, v := range data {
				newData[k] = v.Export()
			}
			if err := handleObjectBody(newData); err != nil {
				return nil, err
			}
		case sobek.ArrayBuffer:
			result.Body = bytes.NewBuffer(data.Bytes())
		case map[string]interface{}:
			if err := handleObjectBody(data); err != nil {
				return nil, err
			}
		case string:
			result.Body = bytes.NewBufferString(data)
		case []byte:
			result.Body = bytes.NewBuffer(data)
		default:
			return nil, fmt.Errorf("unknown request body type %T", body)
		}
	}

	result.Req.Header.Set("User-Agent", state.Options.UserAgent.String)

	if state.CookieJar != nil {
		result.ActiveJar = state.CookieJar
	}

	// TODO: ditch sobek.Value, reflections and Object and use a simple go map and type assertions?
	//nolint: nestif
	if params != nil && !sobek.IsUndefined(params) && !sobek.IsNull(params) {
		params := params.ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "cookies":
				cookiesV := params.Get(k)
				if sobek.IsUndefined(cookiesV) || sobek.IsNull(cookiesV) {
					continue
				}
				cookies := cookiesV.ToObject(rt)
				if cookies == nil {
					continue
				}
				for _, key := range cookies.Keys() {
					cookieV := cookies.Get(key)
					if sobek.IsUndefined(cookieV) || sobek.IsNull(cookieV) {
						continue
					}
					switch cookieV.ExportType() {
					case reflect.TypeOf(map[string]interface{}{}):
						result.Cookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: "", Replace: false}
						cookie := cookieV.ToObject(rt)
						for _, attr := range cookie.Keys() {
							switch strings.ToLower(attr) {
							case "replace":
								result.Cookies[key].Replace = cookie.Get(attr).ToBoolean()
							case "value":
								result.Cookies[key].Value = cookie.Get(attr).String()
							}
						}
					default:
						result.Cookies[key] = &httpext.HTTPRequestCookie{Name: key, Value: cookieV.String(), Replace: false}
					}
				}
			case "headers":
				headersV := params.Get(k)
				if sobek.IsUndefined(headersV) || sobek.IsNull(headersV) {
					continue
				}
				headers := headersV.ToObject(rt)
				if headers == nil {
					continue
				}
				for _, key := range headers.Keys() {
					str := headers.Get(key).String()
					if strings.ToLower(key) == "host" {
						result.Req.Host = str
					}
					result.Req.Header.Set(key, str)
				}
			case "jar":
				jarV := params.Get(k)
				if sobek.IsUndefined(jarV) || sobek.IsNull(jarV) {
					continue
				}
				if v, ok := jarV.Export().(*CookieJar); ok {
					result.ActiveJar = v.Jar
				}
			case "compression":
				algosString := strings.TrimSpace(params.Get(k).ToString().String())
				if algosString == "" {
					continue
				}
				algos := strings.Split(algosString, ",")
				var err error
				result.Compressions = make([]httpext.CompressionType, len(algos))
				for index, algo := range algos {
					algo = strings.TrimSpace(algo)
					result.Compressions[index], err = httpext.CompressionTypeString(algo)
					if err != nil {
						return nil, fmt.Errorf("unknown compression algorithm %s, supported algorithms are %s",
							algo, httpext.CompressionTypeValues())
					}
				}
			case "redirects":
				result.Redirects = null.IntFrom(params.Get(k).ToInteger())
			case "tags":
				if err := common.ApplyCustomUserTags(rt, &result.TagsAndMeta, params.Get(k)); err != nil {
					return nil, fmt.Errorf("invalid HTTP request metric tags: %w", err)
				}
			case "auth":
				result.Auth = params.Get(k).String()
			case "timeout":
				t, err := types.GetDurationValue(params.Get(k).Export())
				if err != nil {
					return nil, fmt.Errorf("invalid timeout value: %w", err)
				}
				result.Timeout = t
			case "throw":
				result.Throw = params.Get(k).ToBoolean()
			case "responseType":
				responseType, err := httpext.ResponseTypeString(params.Get(k).String())
				if err != nil {
					return nil, err
				}
				result.ResponseType = responseType
			case "responseCallback":
				v := params.Get(k).Export()
				if v == nil {
					result.ResponseCallback = nil
				} else if c, ok := v.(*expectedStatuses); ok {
					result.ResponseCallback = c.match
				} else {
					return nil, fmt.Errorf("unsupported responseCallback")
				}
			}
		}
	}

	if result.ActiveJar != nil {
		httpext.SetRequestCookies(result.Req, result.ActiveJar, result.Cookies)
	}

	return result, nil
}

func (c *Client) prepareBatchArray(requests []interface{}) (
	[]httpext.BatchParsedHTTPRequest, []*Response, error,
) {
	reqCount := len(requests)
	batchReqs := make([]httpext.BatchParsedHTTPRequest, reqCount)
	results := make([]*Response, reqCount)

	for i, req := range requests {
		resp := httpext.NewResponse()
		parsedReq, err := c.parseBatchRequest(i, req)
		if err != nil {
			resp.Error = err.Error()
			var k6e httpext.K6Error
			if errors.As(err, &k6e) {
				resp.ErrorCode = int(k6e.Code)
			}
			results[i] = c.responseFromHTTPext(resp)
			return batchReqs, results, err
		}
		batchReqs[i] = httpext.BatchParsedHTTPRequest{
			ParsedHTTPRequest: parsedReq,
			Response:          resp,
		}
		results[i] = c.responseFromHTTPext(resp)
	}

	return batchReqs, results, nil
}

func (c *Client) prepareBatchObject(requests map[string]interface{}) (
	[]httpext.BatchParsedHTTPRequest, map[string]*Response, error,
) {
	reqCount := len(requests)
	batchReqs := make([]httpext.BatchParsedHTTPRequest, reqCount)
	results := make(map[string]*Response, reqCount)

	i := 0
	for key, req := range requests {
		resp := httpext.NewResponse()
		parsedReq, err := c.parseBatchRequest(key, req)
		if err != nil {
			resp.Error = err.Error()
			var k6e httpext.K6Error
			if errors.As(err, &k6e) {
				resp.ErrorCode = int(k6e.Code)
			}
			results[key] = c.responseFromHTTPext(resp)
			return batchReqs, results, err
		}
		batchReqs[i] = httpext.BatchParsedHTTPRequest{
			ParsedHTTPRequest: parsedReq,
			Response:          resp,
		}
		results[key] = c.responseFromHTTPext(resp)
		i++
	}

	return batchReqs, results, nil
}

// Batch makes multiple simultaneous HTTP requests. The provideds reqsV should be an array of request
// objects. Batch returns an array of responses and/or error
func (c *Client) Batch(reqsV ...sobek.Value) (interface{}, error) {
	state := c.moduleInstance.vu.State()
	if state == nil {
		return nil, ErrBatchForbiddenInInitContext
	}

	if len(reqsV) == 0 {
		return nil, fmt.Errorf("no argument was provided to http.batch()")
	} else if len(reqsV) > 1 {
		return nil, fmt.Errorf("http.batch() accepts only an array or an object of requests")
	}
	var (
		err       error
		batchReqs []httpext.BatchParsedHTTPRequest
		results   interface{} // either []*Response or map[string]*Response
	)

	switch v := reqsV[0].Export().(type) {
	case []interface{}:
		batchReqs, results, err = c.prepareBatchArray(v)
	case map[string]interface{}:
		batchReqs, results, err = c.prepareBatchObject(v)
	default:
		return nil, fmt.Errorf("invalid http.batch() argument type %T", v)
	}

	if err != nil {
		if state.Options.Throw.Bool {
			return nil, err
		}
		state.Logger.WithField("error", err).Warn("A batch request failed")
		return results, nil
	}

	reqCount := len(batchReqs)
	errs := httpext.MakeBatchRequests(
		c.moduleInstance.vu.Context(), state, batchReqs, reqCount,
		int(state.Options.Batch.Int64), int(state.Options.BatchPerHost.Int64),
	)

	for i := 0; i < reqCount; i++ {
		if e := <-errs; e != nil && err == nil { // Save only the first error
			err = e
		}
	}
	for _, req := range batchReqs {
		if req.Response != nil {
			c.processResponse(req.Response, req.ParsedHTTPRequest.ResponseType)
		}
	}
	return results, err
}

func (c *Client) parseBatchRequest(key interface{}, val interface{}) (*httpext.ParsedHTTPRequest, error) {
	var (
		method       = http.MethodGet
		ok           bool
		body, reqURL interface{}
		params       sobek.Value
		rt           = c.moduleInstance.vu.Runtime()
	)

	switch data := val.(type) {
	case []interface{}:
		// Handling of ["GET", "http://example.com/"]
		dataLen := len(data)
		if dataLen < 2 {
			return nil, fmt.Errorf("invalid batch request '%#v'", data)
		}
		method, ok = data[0].(string)
		if !ok {
			return nil, fmt.Errorf("invalid method type '%#v'", data[0])
		}
		reqURL = data[1]
		if dataLen > 2 {
			body = data[2]
		}
		if dataLen > 3 {
			params = rt.ToValue(data[3])
		}

	case map[string]interface{}:
		// Handling of {method: "GET", url: "https://test.k6.io"}
		if _, ok := data["url"]; !ok {
			return nil, fmt.Errorf("batch request %v doesn't have a url key", key)
		}

		reqURL = data["url"]
		body = data["body"] // It's fine if it's missing, the map lookup will return

		if newMethod, ok := data["method"]; ok {
			if method, ok = newMethod.(string); !ok {
				return nil, fmt.Errorf("invalid method type '%#v'", newMethod)
			}
			method = strings.ToUpper(method)
			if method == http.MethodGet || method == http.MethodHead {
				body = nil
			}
		}

		if p, ok := data["params"]; ok {
			params = rt.ToValue(p)
		}
	default:
		reqURL = val
	}

	return c.parseRequest(method, reqURL, body, params)
}

func requestContainsFile(data map[string]interface{}) bool {
	for _, v := range data {
		if _, ok := v.(*FileData); ok {
			return true
		}
	}
	return false
}
