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

	"fmt"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules/k6/html"
	"github.com/loadimpact/k6/lib"
	"golang.org/x/crypto/ocsp"
	"net/url"
	"strings"
)

type OCSP struct {
	ProducedAt, ThisUpdate, NextUpdate, RevokedAt int64
	RevocationReason                              string
	Status                                        string
}

type HTTPResponseTimings struct {
	Duration, Blocked, LookingUp, Connecting, Sending, Waiting, Receiving float64
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
	Body           string
	Timings        HTTPResponseTimings
	TLSVersion     string
	TLSCipherSuite string
	OCSP           OCSP `js:"ocsp"`
	Error          string

	cachedJSON goja.Value
}

func (res *HTTPResponse) setTLSInfo(tlsState *tls.ConnectionState) {
	switch tlsState.Version {
	case tls.VersionSSL30:
		res.TLSVersion = SSL_3_0
	case tls.VersionTLS10:
		res.TLSVersion = TLS_1_0
	case tls.VersionTLS11:
		res.TLSVersion = TLS_1_1
	case tls.VersionTLS12:
		res.TLSVersion = TLS_1_2
	}

	res.TLSCipherSuite = lib.SupportedTLSCipherSuitesToString[tlsState.CipherSuite]
	ocspStapledRes := OCSP{Status: OCSP_STATUS_UNKNOWN}

	if ocspRes, err := ocsp.ParseResponse(tlsState.OCSPResponse, nil); err == nil {
		switch ocspRes.Status {
		case ocsp.Good:
			ocspStapledRes.Status = OCSP_STATUS_GOOD
		case ocsp.Revoked:
			ocspStapledRes.Status = OCSP_STATUS_REVOKED
		case ocsp.ServerFailed:
			ocspStapledRes.Status = OCSP_STATUS_SERVER_FAILED
		case ocsp.Unknown:
			ocspStapledRes.Status = OCSP_STATUS_UNKNOWN
		}
		switch ocspRes.RevocationReason {
		case ocsp.Unspecified:
			ocspStapledRes.RevocationReason = OCSP_REASON_UNSPECIFIED
		case ocsp.KeyCompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_KEY_COMPROMISE
		case ocsp.CACompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_CA_COMPROMISE
		case ocsp.AffiliationChanged:
			ocspStapledRes.RevocationReason = OCSP_REASON_AFFILIATION_CHANGED
		case ocsp.Superseded:
			ocspStapledRes.RevocationReason = OCSP_REASON_SUPERSEDED
		case ocsp.CessationOfOperation:
			ocspStapledRes.RevocationReason = OCSP_REASON_CESSATION_OF_OPERATION
		case ocsp.CertificateHold:
			ocspStapledRes.RevocationReason = OCSP_REASON_CERTIFICATE_HOLD
		case ocsp.RemoveFromCRL:
			ocspStapledRes.RevocationReason = OCSP_REASON_REMOVE_FROM_CRL
		case ocsp.PrivilegeWithdrawn:
			ocspStapledRes.RevocationReason = OCSP_REASON_PRIVILEGE_WITHDRAWN
		case ocsp.AACompromise:
			ocspStapledRes.RevocationReason = OCSP_REASON_AA_COMPROMISE
		}
		ocspStapledRes.ProducedAt = ocspRes.ProducedAt.Unix()
		ocspStapledRes.ThisUpdate = ocspRes.ThisUpdate.Unix()
		ocspStapledRes.NextUpdate = ocspRes.NextUpdate.Unix()
		ocspStapledRes.RevokedAt = ocspRes.RevokedAt.Unix()
	}

	res.OCSP = ocspStapledRes
}

func (res *HTTPResponse) Json() goja.Value {
	if res.cachedJSON == nil {
		var v interface{}
		if err := json.Unmarshal([]byte(res.Body), &v); err != nil {
			common.Throw(common.GetRuntime(res.ctx), err)
		}
		res.cachedJSON = common.GetRuntime(res.ctx).ToValue(v)
	}
	return res.cachedJSON
}

func (res *HTTPResponse) Html(selector ...string) html.Selection {
	sel, err := html.HTML{}.ParseHTML(res.ctx, res.Body)
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
	var requestParams goja.Value
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

	actionAttr := form.Attr("action")
	var requestUrl goja.Value
	if actionAttr == goja.Undefined() {
		// Use the url of the response if no action is set
		requestUrl = rt.ToValue(res.URL)
	} else {
		// Resolve the action url from the response url
		responseUrl, _ := url.Parse(res.URL)
		actionUrl, _ := url.Parse(actionAttr.String())
		requestUrl = rt.ToValue(responseUrl.ResolveReference(actionUrl).String())
	}

	// Set the body based on the form values
	body := form.SerializeObject()

	// Set the name + value of the submit button
	submit := form.Find(submitSelector)
	submitName := submit.Attr("name")
	submitValue := submit.Val()
	if submitName != goja.Undefined() && submitValue != goja.Undefined() {
		body[submitName.String()] = submitValue
	}

	// Set the values supplied in the arguments, overriding automatically set values
	if fields != nil {
		for k, v := range fields {
			body[k] = v
		}
	}

	if requestParams == nil {
		return New().Request(res.ctx, requestMethod, requestUrl, rt.ToValue(body))
	} else {
		return New().Request(res.ctx, requestMethod, requestUrl, rt.ToValue(body), requestParams)
	}
}
