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

func NewRoute(logger *log.Logger, networkManager *NetworkManager, request *Request) *Route {
	return &Route{
		logger:         logger,
		networkManager: networkManager,
		request:        request,
		handled:        false,
	}
}

func (r *Route) Request() *Request { return r.request }

func (r *Route) Abort(errorCode string) {
	err := r.startHandling()
	if err != nil {
		r.logger.Errorf("Route:Abort", "rurl:%s err:%s", r.request.URL(), err)
		return
	}

	if errorCode == "" {
		errorCode = "failed"
	}

	r.networkManager.AbortRequest(r.request.interceptionID, errorCode)
}

func (r *Route) Continue() {
	err := r.startHandling()
	if err != nil {
		r.logger.Errorf("Route:Continue", "rurl:%s err:%s", r.request.URL(), err)
		return
	}

	r.networkManager.ContinueRequest(r.request.interceptionID)
}

func (r *Route) startHandling() error {
	if r.handled {
		return fmt.Errorf("route is already handled")
	}
	r.handled = true
	return nil
}
