package api

import "github.com/dop251/goja"

// RequestAPI is the interface of an HTTP request.
type RequestAPI interface {
	AllHeaders() map[string]string
	Failure() goja.Value
	Frame() FrameAPI
	HeaderValue(string) goja.Value
	Headers() map[string]string
	HeadersArray() []HTTPHeaderAPI
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() goja.ArrayBuffer
	PostDataJSON() string
	RedirectedFrom() RequestAPI
	RedirectedTo() RequestAPI
	ResourceType() string
	Response() ResponseAPI
	Size() HTTPMessageSizeAPI
	Timing() goja.Value
	URL() string
}
