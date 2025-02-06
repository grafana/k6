package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/grafana/sobek"
	"github.com/tidwall/gjson"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules/k6/html"
	"go.k6.io/k6/lib/netext/httpext"
)

// Response is a representation of an HTTP response to be returned to the Sobek VM
type Response struct {
	*httpext.Response `js:"-"`
	client            *Client

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
		var v interface{}

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
	for k, v := range fields {
		values[k] = v
	}

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
