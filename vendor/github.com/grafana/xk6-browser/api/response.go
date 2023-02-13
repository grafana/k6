package api

import "github.com/dop251/goja"

// Response is the interface of an HTTP response.
type Response interface {
	AllHeaders() map[string]string
	Body() goja.ArrayBuffer
	Finished() bool
	Frame() Frame
	HeaderValue(string) goja.Value
	HeaderValues(string) []string
	Headers() map[string]string
	HeadersArray() []HTTPHeader
	JSON() goja.Value
	Ok() bool
	Request() Request
	SecurityDetails() goja.Value
	ServerAddr() goja.Value
	Size() HTTPMessageSize
	Status() int64
	StatusText() string
	URL() string
}
