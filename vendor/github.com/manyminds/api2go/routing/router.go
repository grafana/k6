package routing

import "net/http"

// HandlerFunc must contain all params from the route
// in the form key,value
type HandlerFunc func(w http.ResponseWriter, r *http.Request, params map[string]string)

// Routeable allows drop in replacement for api2go's router
// by default, we are using julienschmidt/httprouter
// but you can use any router that has similiar features
// e.g. gin
type Routeable interface {
	// Handler should return the routers main handler, often this is the router itself
	Handler() http.Handler
	// Handle must be implemented to register api2go's default routines
	// to your used router.
	// protocol will be PATCH,OPTIONS,GET,POST,PUT
	// route will be the request route /items/:id where :id means dynamically filled params
	// handler is the handler that will answer to this specific route
	Handle(protocol, route string, handler HandlerFunc)
}
