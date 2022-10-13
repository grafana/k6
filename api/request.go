package api

import "github.com/dop251/goja"

// Request is the interface of an HTTP request.
type Request interface {
	AllHeaders() map[string]string
	Failure() goja.Value
	Frame() Frame
	HeaderValue(string) goja.Value
	Headers() map[string]string
	HeadersArray() []HTTPHeader
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() goja.ArrayBuffer
	PostDataJSON() string
	RedirectedFrom() Request
	RedirectedTo() Request
	ResourceType() string
	Response() Response
	Size() HTTPMessageSize
	Timing() goja.Value
	URL() string
}
