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
	"crypto/tls"
	"encoding/json"
	"errors"

	"fmt"
	"net/url"
	"strings"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/lib/netext"
)

type OCSP struct {
	ProducedAt, ThisUpdate, NextUpdate, RevokedAt int64
	RevocationReason                              string
	Status                                        string
}

type HTTPResponseTimings struct {
	Duration, Blocked, LookingUp, Connecting, TLSHandshaking, Sending, Waiting, Receiving float64
}

type HTTPResponse struct {
	ctx context.Context

	RemoteIP       string
	RemotePort     int
	URL            string
	Status         int
	Proto          string
	Headers        map[string]string
	Cookies        map[string][]*HTTPCookie
	Body           interface{}
	Timings        HTTPResponseTimings
	TLSVersion     string
	TLSCipherSuite string
	OCSP           netext.OCSP `js:"ocsp"`
	Error          string
	Request        HTTPRequest

	cachedJSON goja.Value
}

func (res *HTTPResponse) setTLSInfo(tlsState *tls.ConnectionState) {
	tlsInfo, oscp := netext.ParseTLSConnState(tlsState)
	res.TLSVersion = tlsInfo.Version
	res.TLSCipherSuite = tlsInfo.CipherSuite
	res.OCSP = oscp
}

func (res *HTTPResponse) Json() goja.Value {
	if res.cachedJSON == nil {
		var v interface{}
		var body []byte
		switch b := res.Body.(type) {
		case []byte:
			body = b
		case string:
			body = []byte(b)
		default:
			common.Throw(common.GetRuntime(res.ctx), errors.New("Invalid response type"))
		}
		if err := json.Unmarshal(body, &v); err != nil {
			common.Throw(common.GetRuntime(res.ctx), err)
		}
		res.cachedJSON = common.GetRuntime(res.ctx).ToValue(v)
	}
	return res.cachedJSON
}

func (res *HTTPResponse) Html(selector ...string) html.Selection {
	var body string
	switch b := res.Body.(type) {
	case []byte:
		body = string(b)
	case string:
		body = b
	default:
		common.Throw(common.GetRuntime(res.ctx), errors.New("Invalid response type"))
	}

	sel, err := html.HTML{}.ParseHTML(res.ctx, body)
	if err != nil {
		common.Throw(common.GetRuntime(res.ctx), err)
	}
	sel.URL = res.URL
	if len(selector) > 0 {
		sel = sel.Find(selector[0])
	}
	return sel
}

func (res *HTTPResponse) SubmitForm(args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(res.ctx)

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

	form := res.Html(formSelector)
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

	responseUrl, err := url.Parse(res.URL)
	if err != nil {
		common.Throw(rt, err)
	}

	actionAttr := form.Attr("action")
	var requestUrl *url.URL
	if actionAttr == goja.Undefined() {
		// Use the url of the response if no action is set
		requestUrl = responseUrl
	} else {
		actionUrl, err := url.Parse(actionAttr.String())
		if err != nil {
			common.Throw(rt, err)
		}
		requestUrl = responseUrl.ResolveReference(actionUrl)
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
		requestUrl.RawQuery = q.Encode()
		return New().Request(res.ctx, requestMethod, rt.ToValue(requestUrl.String()), goja.Null(), requestParams)
	}
	return New().Request(res.ctx, requestMethod, rt.ToValue(requestUrl.String()), rt.ToValue(values), requestParams)
}

func (res *HTTPResponse) ClickLink(args ...goja.Value) (*HTTPResponse, error) {
	rt := common.GetRuntime(res.ctx)

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

	responseUrl, err := url.Parse(res.URL)
	if err != nil {
		common.Throw(rt, err)
	}

	link := res.Html(selector)
	if link.Size() == 0 {
		common.Throw(rt, fmt.Errorf("no element found for selector '%s' in response '%s'", selector, res.URL))
	}
	hrefAttr := link.Attr("href")
	if hrefAttr == goja.Undefined() {
		common.Throw(rt, fmt.Errorf("no valid href attribute value found on element '%s' in response '%s'", selector, res.URL))
	}
	hrefUrl, err := url.Parse(hrefAttr.String())
	if err != nil {
		common.Throw(rt, err)
	}
	requestUrl := responseUrl.ResolveReference(hrefUrl)

	return New().Get(res.ctx, rt.ToValue(requestUrl.String()), requestParams)
}
