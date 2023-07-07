package httpext

import (
	"crypto/tls"

	"go.k6.io/k6/lib/netext"
)

// ResponseType is used in the request to specify how the response body should be treated
// The conversion and validation methods are auto-generated with https://github.com/alvaroloes/enumer:
//
//go:generate enumer -type=ResponseType -transform=snake -json -text -trimprefix ResponseType -output response_type_gen.go
type ResponseType uint

const (
	// ResponseTypeText causes k6 to return the response body as a string. It works
	// well for web pages and JSON documents, but it can cause issues with
	// binary files since their data could be lost when they're converted in the
	// UTF-16 strings JavaScript uses.
	// This is the default value for backwards-compatibility, unless the global
	// discardResponseBodies option is enabled.
	ResponseTypeText ResponseType = iota
	// ResponseTypeBinary causes k6 to return the response body as a []byte, suitable
	// for working with binary files without lost data and needless string conversions.
	ResponseTypeBinary
	// ResponseTypeNone causes k6 to fully read the response body while immediately
	// discarding the actual data - k6 would set the body of the returned HTTPResponse
	// to null. This saves CPU and memory and is suitable for HTTP requests that we just
	// want to  measure, but we don't care about their responses' contents. This is the
	// default value for all requests if the global discardResponseBodies is enablled.
	ResponseTypeNone
)

// ResponseTimings is a struct to put all timings for a given HTTP response/request
type ResponseTimings struct {
	Duration       float64 `json:"duration"`
	Blocked        float64 `json:"blocked"`
	LookingUp      float64 `json:"looking_up"`
	Connecting     float64 `json:"connecting"`
	TLSHandshaking float64 `json:"tls_handshaking"`
	Sending        float64 `json:"sending"`
	Waiting        float64 `json:"waiting"`
	Receiving      float64 `json:"receiving"`
}

// HTTPCookie is a representation of an http cookies used in the Response object
type HTTPCookie struct {
	Name, Value, Domain, Path string
	HTTPOnly, Secure          bool
	MaxAge                    int
	Expires                   int64
}

// Response is a representation of an HTTP response
type Response struct {
	RemoteIP       string                   `json:"remote_ip"`
	RemotePort     int                      `json:"remote_port"`
	URL            string                   `json:"url"`
	Status         int                      `json:"status"`
	StatusText     string                   `json:"status_text"`
	Proto          string                   `json:"proto"`
	Headers        map[string]string        `json:"headers"`
	Cookies        map[string][]*HTTPCookie `json:"cookies"`
	Body           interface{}              `json:"body"`
	Timings        ResponseTimings          `json:"timings"`
	TLSVersion     string                   `json:"tls_version"`
	TLSCipherSuite string                   `json:"tls_cipher_suite"`
	OCSP           netext.OCSP              `json:"ocsp"`
	Error          string                   `json:"error"`
	ErrorCode      int                      `json:"error_code"`
	Request        *Request                 `json:"request"`
}

// NewResponse returns an empty Response instance.
func NewResponse() *Response {
	return &Response{
		Body: []byte{},
	}
}

func (res *Response) setTLSInfo(tlsState *tls.ConnectionState) {
	tlsInfo, oscp := netext.ParseTLSConnState(tlsState)
	res.TLSVersion = tlsInfo.Version
	res.TLSCipherSuite = tlsInfo.CipherSuite
	res.OCSP = oscp
}
