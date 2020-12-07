package routing

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// HTTPRouter default router implementation for api2go
type HTTPRouter struct {
	router *httprouter.Router
}

// Handle each method like before and wrap them into julienschmidt handler func style
func (h HTTPRouter) Handle(protocol, route string, handler HandlerFunc) {
	wrappedCallback := func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		params := map[string]string{}
		for _, p := range ps {
			params[p.Key] = p.Value
		}

		handler(w, r, params, make(map[string]interface{}))
	}

	h.router.Handle(protocol, route, wrappedCallback)
}

// Handler returns the router
func (h HTTPRouter) Handler() http.Handler {
	return h.router
}

// SetRedirectTrailingSlash wraps this internal functionality of
// the julienschmidt router.
func (h *HTTPRouter) SetRedirectTrailingSlash(enabled bool) {
	h.router.RedirectTrailingSlash = enabled
}

// GetRouteParameter implemention will extract the param the julienschmidt way
func (h HTTPRouter) GetRouteParameter(r http.Request, param string) string {
	path := httprouter.CleanPath(r.URL.Path)
	_, params, _ := h.router.Lookup(r.Method, path)
	return params.ByName(param)
}

// NewHTTPRouter returns a new instance of julienschmidt/httprouter
// this is the default router when using api2go
func NewHTTPRouter(prefix string, notAllowedHandler http.Handler) Routeable {
	router := httprouter.New()
	router.HandleMethodNotAllowed = true
	router.MethodNotAllowed = notAllowedHandler
	return &HTTPRouter{router: router}
}
