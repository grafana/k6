/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dop251/goja"
	"github.com/tidwall/gjson"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules/k6/html"
	"go.k6.io/k6/lib/netext/httpext"
)

// Response is a representation of an HTTP response to be returned to the goja VM
type Response struct {
	*httpext.Response `js:"-"`
	h                 *HTTP

	cachedJSON    interface{}
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

// processResponse stores the body as an ArrayBuffer if indicated by
// respType. This is done here instead of in httpext.readResponseBody to avoid
// a reverse dependency on js/common or goja.
func processResponse(ctx context.Context, resp *httpext.Response, respType httpext.ResponseType) {
	if respType == httpext.ResponseTypeBinary && resp.Body != nil {
		rt := common.GetRuntime(ctx)
		resp.Body = rt.NewArrayBuffer(resp.Body.([]byte))
	}
}

func (h *HTTP) responseFromHttpext(resp *httpext.Response) *Response {
	return &Response{Response: resp, h: h, cachedJSON: nil, validatedJSON: false}
}

// HTML returns the body as an html.Selection
func (res *Response) HTML(selector ...string) html.Selection {
	body, err := common.ToString(res.Body)
	if err != nil {
		common.Throw(common.GetRuntime(res.GetCtx()), err)
	}

	sel, err := html.HTML{}.ParseHTML(res.GetCtx(), body)
	if err != nil {
		common.Throw(common.GetRuntime(res.GetCtx()), err)
	}
	sel.URL = res.URL
	if len(selector) > 0 {
		sel = sel.Find(selector[0])
	}
	return sel
}

// JSON parses the body of a response as JSON and returns it to the goja VM.
func (res *Response) JSON(selector ...string) goja.Value {
	rt := common.GetRuntime(res.GetCtx())
	hasSelector := len(selector) > 0
	if res.cachedJSON == nil || hasSelector { //nolint:nestif
		var v interface{}

		body, err := common.ToBytes(res.Body)
		if err != nil {
			common.Throw(rt, err)
		}

		if hasSelector {
			if !res.validatedJSON {
				if !gjson.ValidBytes(body) {
					return goja.Undefined()
				}
				res.validatedJSON = true
			}

			result := gjson.GetBytes(body, selector[0])

			if !result.Exists() {
				return goja.Undefined()
			}
			return rt.ToValue(result.Value())
		}

		if err := json.Unmarshal(body, &v); err != nil {
			var syntaxError *json.SyntaxError
			if errors.As(err, &syntaxError) {
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

	for i, b := range str {
		if b == lf {
			line++
			character = 0
		}
		character++
		if i == offset {
			break
		}
	}

	return jsonError{line: line, character: character, err: err}
}

// SubmitForm parses the body as an html looking for a from and then submitting it
// TODO: document the actual arguments that can be provided
func (res *Response) SubmitForm(args ...goja.Value) (*Response, error) {
	rt := common.GetRuntime(res.GetCtx())

	formSelector := "form"
	submitSelector := "[type=\"submit\"]"
	var fields map[string]goja.Value
	requestParams := goja.Null()
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
	if methodAttr == goja.Undefined() {
		// Use GET by default
		requestMethod = HTTP_METHOD_GET
	} else {
		requestMethod = strings.ToUpper(methodAttr.String())
	}

	responseURL, err := url.Parse(res.URL)
	if err != nil {
		common.Throw(rt, err)
	}

	actionAttr := form.Attr("action")
	var requestURL *url.URL
	if actionAttr == goja.Undefined() {
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
	if submitName != goja.Undefined() && submitValue != goja.Undefined() {
		values[submitName.String()] = submitValue
	}

	// Set the values supplied in the arguments, overriding automatically set values
	for k, v := range fields {
		values[k] = v
	}

	if requestMethod == HTTP_METHOD_GET {
		q := url.Values{}
		for k, v := range values {
			q.Add(k, v.String())
		}
		requestURL.RawQuery = q.Encode()
		return res.h.Request(res.GetCtx(), requestMethod, rt.ToValue(requestURL.String()), goja.Null(), requestParams)
	}
	return res.h.Request(res.GetCtx(), requestMethod, rt.ToValue(requestURL.String()), rt.ToValue(values), requestParams)
}

// ClickLink parses the body as an html, looks for a link and than makes a request as if the link was
// clicked
func (res *Response) ClickLink(args ...goja.Value) (*Response, error) {
	rt := common.GetRuntime(res.GetCtx())

	selector := "a[href]"
	requestParams := goja.Null()
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
	if hrefAttr == goja.Undefined() {
		common.Throw(rt, fmt.Errorf("no valid href attribute value found on element '%s' in response '%s'", selector, res.URL))
	}
	hrefURL, err := url.Parse(hrefAttr.String())
	if err != nil {
		common.Throw(rt, err)
	}
	requestURL := responseURL.ResolveReference(hrefURL)

	return res.h.Get(res.GetCtx(), rt.ToValue(requestURL.String()), requestParams)
}
