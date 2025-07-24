package common

import (
	"fmt"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// Route allows to handle a request.
type Route struct {
	logger         *log.Logger
	networkManager *NetworkManager

	request *Request
	handled bool
}

// FulfillOptions are response fields that can be set when fulfilling a request.
type FulfillOptions struct {
	Body        string
	ContentType string
	Headers     []HTTPHeader
	Status      int64
}

// NewRoute creates a new Route that allows to modify a request's behavior.
func NewRoute(logger *log.Logger, networkManager *NetworkManager, request *Request) *Route {
	return &Route{
		logger:         logger,
		networkManager: networkManager,
		request:        request,
		handled:        false,
	}
}

// Request returns the request associated with the route.
func (r *Route) Request() *Request { return r.request }

// Abort aborts the request with the given error code.
func (r *Route) Abort(errorCode string) error {
	err := r.startHandling()
	if err != nil {
		return err
	}

	if errorCode == "" {
		errorCode = "failed"
	}

	return r.networkManager.AbortRequest(r.request.interceptionID, errorCode)
}

// Continue continues the request.
func (r *Route) Continue() error {
	err := r.startHandling()
	if err != nil {
		return err
	}

	return r.networkManager.ContinueRequest(r.request.interceptionID)
}

// Fulfill fulfills the request with the given options for the response.
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
