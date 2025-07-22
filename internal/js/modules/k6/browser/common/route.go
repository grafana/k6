package common

import (
	"fmt"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// Route allows to handle a request
type Route struct {
	logger         *log.Logger
	networkManager *NetworkManager

	request *Request
	handled bool
}

type ContinueOptions struct {
	Headers  []HTTPHeader
	Method   string
	PostData string
	URL      string
}

type FulfillOptions struct {
	Body        string
	ContentType string
	Headers     []HTTPHeader
	Status      int64
}

func NewRoute(logger *log.Logger, networkManager *NetworkManager, request *Request) *Route {
	return &Route{
		logger:         logger,
		networkManager: networkManager,
		request:        request,
		handled:        false,
	}
}

func (r *Route) Request() *Request { return r.request }

func (r *Route) Abort(errorCode string) error {
	r.logger.Infof("Route:Abort", "rurl:%s errorCode:%s", r.request.URL(), errorCode)
	err := r.startHandling()
	if err != nil {
		return err
	}

	if errorCode == "" {
		errorCode = "failed"
	}

	return r.networkManager.AbortRequest(r.request.interceptionID, errorCode)
}

func (r *Route) Continue(opts *ContinueOptions) error {
	err := r.startHandling()
	if err != nil {
		return err
	}

	return r.networkManager.ContinueRequest(r.request.interceptionID, opts, r.request.HeadersArray())
}

func (r *Route) Fulfill(opts *FulfillOptions) error {
	err := r.startHandling()
	if err != nil {
		return err
	}

	return r.networkManager.FulfillRequest(r.request, opts)
}

func (r *Route) startHandling() error {
	if r.handled {
		return fmt.Errorf("route is already handled")
	}
	r.handled = true
	return nil
}
