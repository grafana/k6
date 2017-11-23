package api2go

import "net/http"

type callbackResolver struct {
	callback func(r http.Request) string
	r        http.Request
}

// NewCallbackResolver handles each resolve via
// your provided callback func
func NewCallbackResolver(callback func(http.Request) string) URLResolver {
	return &callbackResolver{callback: callback}
}

// GetBaseURL calls the callback given in the constructor method
// to implement `URLResolver`
func (c callbackResolver) GetBaseURL() string {
	return c.callback(c.r)
}

// SetRequest to implement `RequestAwareURLResolver`
func (c *callbackResolver) SetRequest(r http.Request) {
	c.r = r
}

// staticResolver is only used
// for backwards compatible reasons
// and might be removed in the future
type staticResolver struct {
	baseURL string
}

func (s staticResolver) GetBaseURL() string {
	return s.baseURL
}

// NewStaticResolver returns a simple resolver that
// will always answer with the same url
func NewStaticResolver(baseURL string) URLResolver {
	return &staticResolver{baseURL: baseURL}
}
