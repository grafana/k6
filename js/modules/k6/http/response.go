package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"github.com/tidwall/gjson"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules/k6/html"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

// Response is a representation of an HTTP response to be returned to the Sobek VM
type Response struct {
	*httpext.Response `js:"-"`
	client            *Client

	cachedJSON    any
	validatedJSON bool
}

type jsonError struct {
	line      int
	character int
	err       error
}

func (j jsonError) Error() string {
	errMessage := "cannot parse json due to an error at line"
	return fmt.Sprintf("%s %d, character %d , error: %v", errMessage, j.line, j.character, j.err)
}

// HTML returns the body as an html.Selection
func (res *Response) HTML(selector ...string) html.Selection {
	rt := res.client.moduleInstance.vu.Runtime()
	if res.Body == nil {
		err := fmt.Errorf("the body is null so we can't transform it to HTML" +
			" - this likely was because of a request error getting the response")
		common.Throw(rt, err)
	}

	body, err := common.ToString(res.Body)
	if err != nil {
		common.Throw(rt, err)
	}

	sel, err := html.ParseHTML(rt, body)
	if err != nil {
		common.Throw(rt, err)
	}
	sel.URL = res.URL
	if len(selector) > 0 {
		sel = sel.Find(selector[0])
	}
	return sel
}

// JSON parses the body of a response as JSON and returns it to the Sobek VM.
func (res *Response) JSON(selector ...string) sobek.Value {
	rt := res.client.moduleInstance.vu.Runtime()

	if res.Body == nil {
		err := fmt.Errorf("the body is null so we can't transform it to JSON" +
			" - this likely was because of a request error getting the response")
		common.Throw(rt, err)
	}

	hasSelector := len(selector) > 0
	if res.cachedJSON == nil || hasSelector { //nolint:nestif
		var v any

		body, err := common.ToBytes(res.Body)
		if err != nil {
			common.Throw(rt, err)
		}

		if hasSelector {
			if !res.validatedJSON {
				if !gjson.ValidBytes(body) {
					return sobek.Undefined()
				}
				res.validatedJSON = true
			}

			result := gjson.GetBytes(body, selector[0])

			if !result.Exists() {
				return sobek.Undefined()
			}
			return rt.ToValue(result.Value())
		}

		if err := json.Unmarshal(body, &v); err != nil {
			if syntaxError, ok := errors.AsType[*json.SyntaxError](err); ok {
				err = checkErrorInJSON(body, int(syntaxError.Offset), err)
			}
			common.Throw(rt, err)
		}
		res.validatedJSON = true
		res.cachedJSON = v
	}

	return rt.ToValue(res.cachedJSON)
}

func checkErrorInJSON(input []byte, offset int, err error) error {
	lf := '\n'
	str := string(input)

	// Humans tend to count from 1.
	line := 1
	character := 0

	// SyntaxError.Offset is the number of bytes read (1-based).
	if offset > 0 {
		offset--
	}

	for i, b := range str {
		if b == lf {
			line++
			character = 0
		} else {
			character++
		}

		if i >= offset {
			if b == lf {
				character = 1
			}
			break
		}
	}

	return jsonError{line: line, character: character, err: err}
}

// SubmitForm parses the body as an html looking for a from and then submitting it
// TODO: document the actual arguments that can be provided
func (res *Response) SubmitForm(args ...sobek.Value) (*Response, error) {
	rt := res.client.moduleInstance.vu.Runtime()

	formSelector := "form"
	submitSelector := "[type=\"submit\"]"
	var fields map[string]sobek.Value
	requestParams := sobek.Null()
	if len(args) > 0 {
		params := args[0].ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "formSelector":
				formSelector = params.Get(k).String()
			case "submitSelector":
				submitSelector = params.Get(k).String()
			case "fields":
				if rt.ExportTo(params.Get(k), &fields) != nil {
					fields = nil
				}
			case "params":
				requestParams = params.Get(k)
			}
		}
	}

	form := res.HTML(formSelector)
	if form.Size() == 0 {
		common.Throw(rt, fmt.Errorf("no form found for selector '%s' in response '%s'", formSelector, res.URL))
	}

	methodAttr := form.Attr("method")
	var requestMethod string
	if methodAttr == sobek.Undefined() {
		// Use GET by default
		requestMethod = http.MethodGet
	} else {
		requestMethod = strings.ToUpper(methodAttr.String())
	}

	responseURL, err := url.Parse(res.URL)
	if err != nil {
		common.Throw(rt, err)
	}

	actionAttr := form.Attr("action")
	var requestURL *url.URL
	if actionAttr == sobek.Undefined() {
		// Use the url of the response if no action is set
		requestURL = responseURL
	} else {
		actionURL, err := url.Parse(actionAttr.String())
		if err != nil {
			common.Throw(rt, err)
		}
		requestURL = responseURL.ResolveReference(actionURL)
	}

	// Set the body based on the form values
	values := form.SerializeObject()

	// Set the name + value of the submit button
	submit := form.Find(submitSelector)
	submitName := submit.Attr("name")
	submitValue := submit.Val()
	if submitName != sobek.Undefined() && submitValue != sobek.Undefined() {
		values[submitName.String()] = submitValue
	}

	// Set the values supplied in the arguments, overriding automatically set values
	maps.Copy(values, fields)

	if requestMethod == http.MethodGet {
		q := url.Values{}
		for k, v := range values {
			q.Add(k, v.String())
		}
		requestURL.RawQuery = q.Encode()
		return res.client.Request(requestMethod, rt.ToValue(requestURL.String()), sobek.Null(), requestParams)
	}
	return res.client.Request(
		requestMethod, rt.ToValue(requestURL.String()),
		rt.ToValue(values), requestParams,
	)
}

// ClickLink parses the body as an html, looks for a link and than makes a request as if the link was
// clicked
func (res *Response) ClickLink(args ...sobek.Value) (*Response, error) {
	rt := res.client.moduleInstance.vu.Runtime()

	selector := "a[href]"
	requestParams := sobek.Null()
	if len(args) > 0 {
		params := args[0].ToObject(rt)
		for _, k := range params.Keys() {
			switch k {
			case "selector":
				selector = params.Get(k).String()
			case "params":
				requestParams = params.Get(k)
			}
		}
	}

	responseURL, err := url.Parse(res.URL)
	if err != nil {
		common.Throw(rt, err)
	}

	link := res.HTML(selector)
	if link.Size() == 0 {
		common.Throw(rt, fmt.Errorf("no element found for selector '%s' in response '%s'", selector, res.URL))
	}
	hrefAttr := link.Attr("href")
	if hrefAttr == sobek.Undefined() {
		errmsg := fmt.Errorf("no valid href attribute value found on element '%s' in response '%s'", selector, res.URL)
		common.Throw(rt, errmsg)
	}
	hrefURL, err := url.Parse(hrefAttr.String())
	if err != nil {
		common.Throw(rt, err)
	}
	requestURL := responseURL.ResolveReference(hrefURL)

	return res.client.Request(http.MethodGet, rt.ToValue(requestURL.String()), sobek.Undefined(), requestParams)
}

// Save writes the response URL, status, headers, and body as formatted JSON to the given
// filePath (or an auto-generated path in ./saved_responses/ if omitted) and emits an
// http_saved_responses metric sample containing the response headers and body.
func (res *Response) Save(args ...string) (string, error) {
	now := time.Now()
	filePath := ""
	if len(args) > 0 {
		filePath = strings.TrimSpace(args[0])
	}
	if filePath == "" {
		filePath = fmt.Sprintf("saved_responses/response-%d-status-%d.json", now.UnixNano(), res.Status)
	}

	var bodyVal any
	var bodyStr string
	switch b := res.Body.(type) {
	case nil:
		bodyVal = nil
		bodyStr = ""
	case sobek.ArrayBuffer:
		bodyStr = string(b.Bytes())
		bodyVal = bodyStr
	case []byte:
		bodyStr = string(b)
		bodyVal = bodyStr
	case string:
		bodyStr = b
		bodyVal = b
	default:
		bodyStr = fmt.Sprintf("%v", b)
		bodyVal = b
	}

	payload := map[string]any{
		"timestamp":   now.Format(time.RFC3339Nano),
		"url":         res.URL,
		"status":      res.Status,
		"status_text": res.StatusText,
		"headers":     res.Headers,
		"body":        bodyVal,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if dir := filepath.Dir(filePath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", err
		}
	}

	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		return "", err
	}

	if res.client != nil && res.client.moduleInstance != nil {
		if state := res.client.moduleInstance.vu.State(); state != nil &&
			state.BuiltinMetrics != nil && state.BuiltinMetrics.HTTPSavedResponses != nil {
			headersJSON, _ := json.Marshal(res.Headers)
			tagsAndMeta := state.Tags.GetCurrentValues()
			statusStr := strconv.Itoa(res.Status)
			tags := tagsAndMeta.Tags.
				With("url", res.URL).
				With("status", statusStr)

			meta := maps.Clone(tagsAndMeta.Metadata)
			if meta == nil {
				meta = make(map[string]string)
			}
			meta["url"] = res.URL
			meta["status"] = statusStr
			meta["status_text"] = res.StatusText
			meta["response_headers"] = string(headersJSON)
			meta["response_body"] = bodyStr
			meta["saved_file"] = filePath

			metrics.PushIfNotDone(res.client.moduleInstance.vu.Context(), state.Samples, metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: state.BuiltinMetrics.HTTPSavedResponses,
					Tags:   tags,
				},
				Time:     now,
				Value:    1,
				Metadata: meta,
			})
		}
	}

	return filePath, nil
}
