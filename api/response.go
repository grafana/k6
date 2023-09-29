package api

import "github.com/dop251/goja"

// ResponseAPI is the interface of an HTTP response.
type ResponseAPI interface {
	AllHeaders() map[string]string
	Body() goja.ArrayBuffer
	Finished() bool
	Frame() FrameAPI
	HeaderValue(string) goja.Value
	HeaderValues(string) []string
	Headers() map[string]string
	HeadersArray() []HTTPHeaderAPI
	JSON() goja.Value
	Ok() bool
	Request() RequestAPI
	SecurityDetails() goja.Value
	ServerAddr() goja.Value
	Size() HTTPMessageSizeAPI
	Status() int64
	StatusText() string
	URL() string
}
