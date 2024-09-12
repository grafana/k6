package http

import (
	"net/http"
	"net/http/cookiejar"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/netext/httpext"
)

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create HTTP module instances for each VU.
//
// TODO: add sync.Once for all of the deprecation warnings we might want to do
// for the old k6/http APIs here, so they are shown only once in a test run.
type RootModule struct{}

// ModuleInstance represents an instance of the HTTP module for every VU.
type ModuleInstance struct {
	vu            modules.VU
	rootModule    *RootModule
	defaultClient *Client
	exports       *sobek.Object
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new HTTP RootModule.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance returns an HTTP module instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()
	mi := &ModuleInstance{
		vu:         vu,
		rootModule: r,
		exports:    rt.NewObject(),
	}
	mi.defineConstants()

	mi.defaultClient = &Client{
		// TODO: configure this from lib.Options and get rid of some of the
		// things in the VU State struct that should be here. See
		// https://github.com/grafana/k6/issues/2293
		moduleInstance:   mi,
		responseCallback: defaultExpectedStatuses.match,
	}

	mustExport := func(name string, value interface{}) {
		if err := mi.exports.Set(name, value); err != nil {
			common.Throw(rt, err)
		}
	}

	mustExport("url", mi.URL)
	mustExport("CookieJar", mi.newCookieJar)
	mustExport("cookieJar", mi.getVUCookieJar)
	mustExport("file", mi.file) // TODO: deprecate or refactor?

	// TODO: refactor so the Client actually has better APIs and these are
	// wrappers (facades) that convert the old k6 idiosyncratic APIs to the new
	// proper Client ones that accept Request objects and don't suck
	mustExport("get", func(url sobek.Value, args ...sobek.Value) (*Response, error) {
		// http.get(url, params) doesn't have a body argument, so we add undefined
		// as the third argument to http.request(method, url, body, params)
		args = append([]sobek.Value{sobek.Undefined()}, args...)
		return mi.defaultClient.Request(http.MethodGet, url, args...)
	})
	mustExport("head", func(url sobek.Value, args ...sobek.Value) (*Response, error) {
		// http.head(url, params) doesn't have a body argument, so we add undefined
		// as the third argument to http.request(method, url, body, params)
		args = append([]sobek.Value{sobek.Undefined()}, args...)
		return mi.defaultClient.Request(http.MethodHead, url, args...)
	})
	mustExport("post", mi.defaultClient.getMethodClosure(http.MethodPost))
	mustExport("put", mi.defaultClient.getMethodClosure(http.MethodPut))
	mustExport("patch", mi.defaultClient.getMethodClosure(http.MethodPatch))
	mustExport("del", mi.defaultClient.getMethodClosure(http.MethodDelete))
	mustExport("options", mi.defaultClient.getMethodClosure(http.MethodOptions))
	mustExport("request", mi.defaultClient.Request)
	mustExport("asyncRequest", mi.defaultClient.asyncRequest)
	mustExport("batch", mi.defaultClient.Batch)
	mustExport("setResponseCallback", mi.defaultClient.SetResponseCallback)

	mustExport("expectedStatuses", mi.expectedStatuses) // TODO: refactor?

	// TODO: actually expose the default client as k6/http.defaultClient when we
	// have a better HTTP API (e.g. proper Client constructor, an actual Request
	// object, custom Transport implementations you can pass the Client, etc.).
	// This will allow us to find solutions to many of the issues with the
	// current HTTP API that plague us:
	// https://github.com/grafana/k6/issues?q=is%3Aopen+is%3Aissue+label%3Anew-http

	return mi
}

// Exports returns the JS values this module exports.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
		// TODO: add new HTTP APIs like Client, Request (see above comment in
		// NewModuleInstance()), etc. as named exports?
	}
}

func (mi *ModuleInstance) defineConstants() {
	rt := mi.vu.Runtime()
	mustAddProp := func(name, val string) {
		err := mi.exports.DefineDataProperty(
			name, rt.ToValue(val), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_TRUE,
		)
		if err != nil {
			common.Throw(rt, err)
		}
	}
	mustAddProp("TLS_1_0", netext.TLS_1_0)
	mustAddProp("TLS_1_1", netext.TLS_1_1)
	mustAddProp("TLS_1_2", netext.TLS_1_2)
	mustAddProp("TLS_1_3", netext.TLS_1_3)
	mustAddProp("OCSP_STATUS_GOOD", netext.OCSP_STATUS_GOOD)
	mustAddProp("OCSP_STATUS_REVOKED", netext.OCSP_STATUS_REVOKED)
	mustAddProp("OCSP_STATUS_SERVER_FAILED", netext.OCSP_STATUS_SERVER_FAILED)
	mustAddProp("OCSP_STATUS_UNKNOWN", netext.OCSP_STATUS_UNKNOWN)
	mustAddProp("OCSP_REASON_UNSPECIFIED", netext.OCSP_REASON_UNSPECIFIED)
	mustAddProp("OCSP_REASON_KEY_COMPROMISE", netext.OCSP_REASON_KEY_COMPROMISE)
	mustAddProp("OCSP_REASON_CA_COMPROMISE", netext.OCSP_REASON_CA_COMPROMISE)
	mustAddProp("OCSP_REASON_AFFILIATION_CHANGED", netext.OCSP_REASON_AFFILIATION_CHANGED)
	mustAddProp("OCSP_REASON_SUPERSEDED", netext.OCSP_REASON_SUPERSEDED)
	mustAddProp("OCSP_REASON_CESSATION_OF_OPERATION", netext.OCSP_REASON_CESSATION_OF_OPERATION)
	mustAddProp("OCSP_REASON_CERTIFICATE_HOLD", netext.OCSP_REASON_CERTIFICATE_HOLD)
	mustAddProp("OCSP_REASON_REMOVE_FROM_CRL", netext.OCSP_REASON_REMOVE_FROM_CRL)
	mustAddProp("OCSP_REASON_PRIVILEGE_WITHDRAWN", netext.OCSP_REASON_PRIVILEGE_WITHDRAWN)
	mustAddProp("OCSP_REASON_AA_COMPROMISE", netext.OCSP_REASON_AA_COMPROMISE)
}

func (mi *ModuleInstance) newCookieJar(_ sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	jar, err := cookiejar.New(nil)
	if err != nil {
		common.Throw(rt, err)
	}
	return rt.ToValue(&CookieJar{mi, jar}).ToObject(rt)
}

// getVUCookieJar returns the active cookie jar for the current VU.
func (mi *ModuleInstance) getVUCookieJar(_ sobek.FunctionCall) sobek.Value {
	rt := mi.vu.Runtime()
	if state := mi.vu.State(); state != nil {
		return rt.ToValue(&CookieJar{mi, state.CookieJar})
	}
	common.Throw(rt, ErrJarForbiddenInInitContext)
	return nil
}

// URL creates a new URL wrapper from the provided parts.
func (mi *ModuleInstance) URL(parts []string, pieces ...string) (httpext.URL, error) {
	var name, urlstr string
	for i, part := range parts {
		name += part
		urlstr += part
		if i < len(pieces) {
			name += "${}"
			urlstr += pieces[i]
		}
	}
	return httpext.NewURL(urlstr, name)
}

// Client represents a stand-alone HTTP client.
//
// TODO: move to its own file
type Client struct {
	moduleInstance   *ModuleInstance
	responseCallback func(int) bool
}
