package har

import (
	"time"
)

// HAR is the top level object of a HAR log.
type HAR struct {
	Log *Log `json:"log"`
}

// Log is the HAR HTTP request and response log.
type Log struct {
	// Version number of the HAR format.
	Version string `json:"version"`
	// Creator holds information about the log creator application.
	Creator *Creator `json:"creator"`
	// Browser
	Browser *Browser `json:"browser,omitempty"`
	// Pages
	Pages []Page `json:"pages,omitempty"`
	// Entries is a list containing requests and responses.
	Entries []*Entry `json:"entries"`
	//
	Comment string `json:"comment,omitempty"`
}

// Creator is the program responsible for generating the log. Martian, in this case.
type Creator struct {
	// Name of the log creator application.
	Name string `json:"name"`
	// Version of the log creator application.
	Version string `json:"version"`
}

// Browser that created the log
type Browser struct {
	// Required. The name of the browser that created the log.
	Name string `json:"name"`
	// Required. The version number of the browser that created the log.
	Version string `json:"version"`
	// Optional. A comment provided by the user or the browser.
	Comment string `json:"comment"`
}

// Page object for every exported web page and one <entry> object for every HTTP request.
// In case when an HTTP trace tool isn't able to group requests by a page,
// the <pages> object is empty and individual requests doesn't have a parent page.
type Page struct {
	/* There is one <page> object for every exported web page and one <entry>
	   object for every HTTP request. In case when an HTTP trace tool isn't able to
	   group requests by a page, the <pages> object is empty and individual
	   requests doesn't have a parent page.
	*/

	// Date and time stamp for the beginning of the page load
	// (ISO 8601 YYYY-MM-DDThh:mm:ss.sTZD, e.g. 2009-07-24T19:20:30.45+01:00).
	StartedDateTime time.Time `json:"startedDateTime"`
	// Unique identifier of a page within the . Entries use it to refer the parent page.
	ID string `json:"id"`
	// Page title.
	Title string `json:"title"`
	// (new in 1.2) A comment provided by the user or the application.
	Comment string `json:"comment,omitempty"`
}

// Entry is a individual log entry for a request or response.
type Entry struct {
	Pageref string `json:"pageref,omitempty"`
	// ID is the unique ID for the entry.
	ID string `json:"_id"`
	// StartedDateTime is the date and time stamp of the request start (ISO 8601).
	StartedDateTime time.Time `json:"startedDateTime"`
	// Time is the total elapsed time of the request in milliseconds.
	Time float32 `json:"time"`
	// Request contains the detailed information about the request.
	Request *Request `json:"request"`
	// Response contains the detailed information about the response.
	Response *Response `json:"response,omitempty"`
	// Cache contains information about a request coming from browser cache.
	Cache *Cache `json:"cache"`
	// Timings describes various phases within request-response round trip. All
	// times are specified in milliseconds.
	Timings *Timings `json:"timings"`
}

// Request holds data about an individual HTTP request.
type Request struct {
	// Method is the request method (GET, POST, ...).
	Method string `json:"method"`
	// URL is the absolute URL of the request (fragments are not included).
	URL string `json:"url"`
	// HTTPVersion is the Request HTTP version (HTTP/1.1).
	HTTPVersion string `json:"httpVersion"`
	// Cookies is a list of cookies.
	Cookies []Cookie `json:"cookies"`
	// Headers is a list of headers.
	Headers []Header `json:"headers"`
	// QueryString is a list of query parameters.
	QueryString []QueryString `json:"queryString"`
	// PostData is the posted data information.
	PostData *PostData `json:"postData,omitempty"`
	// HeaderSize is the Total number of bytes from the start of the HTTP request
	// message until (and including) the double CLRF before the body. Set to -1
	// if the info is not available.
	HeadersSize int64 `json:"headersSize"`
	// BodySize is the size of the request body (POST data payload) in bytes. Set
	// to -1 if the info is not available.
	BodySize int64 `json:"bodySize"`
	// (new in 1.2) A comment provided by the user or the application.
	Comment string `json:"comment"`
}

// Response holds data about an individual HTTP response.
type Response struct {
	// Status is the response status code.
	Status int `json:"status"`
	// StatusText is the response status description.
	StatusText string `json:"statusText"`
	// HTTPVersion is the Response HTTP version (HTTP/1.1).
	HTTPVersion string `json:"httpVersion"`
	// Cookies is a list of cookies.
	Cookies []Cookie `json:"cookies"`
	// Headers is a list of headers.
	Headers []Header `json:"headers"`
	// Content contains the details of the response body.
	Content *Content `json:"content"`
	// RedirectURL is the target URL from the Location response header.
	RedirectURL string `json:"redirectURL"`
	// HeadersSize is the total number of bytes from the start of the HTTP
	// request message until (and including) the double CLRF before the body.
	// Set to -1 if the info is not available.
	HeadersSize int64 `json:"headersSize"`
	// BodySize is the size of the request body (POST data payload) in bytes. Set
	// to -1 if the info is not available.
	BodySize int64 `json:"bodySize"`
}

// Cache contains information about a request coming from browser cache.
type Cache struct {
	// Has no fields as they are not supported, but HAR requires the "cache"
	// object to exist.
}

// Timings describes various phases within request-response round trip. All
// times are specified in milliseconds
type Timings struct {
	// Send is the time required to send HTTP request to the server.
	Send float32 `json:"send"`
	// Wait is the time spent waiting for a response from the server.
	Wait float32 `json:"wait"`
	// Receive is the time required to read entire response from server or cache.
	Receive float32 `json:"receive"`
}

// Cookie is the data about a cookie on a request or response.
type Cookie struct {
	// Name is the cookie name.
	Name string `json:"name"`
	// Value is the cookie value.
	Value string `json:"value"`
	// Path is the path pertaining to the cookie.
	Path string `json:"path,omitempty"`
	// Domain is the host of the cookie.
	Domain string `json:"domain,omitempty"`
	// Expires contains cookie expiration time.
	Expires time.Time `json:"-"`
	// Expires8601 contains cookie expiration time in ISO 8601 format.
	Expires8601 string `json:"expires,omitempty"`
	// HTTPOnly is set to true if the cookie is HTTP only, false otherwise.
	HTTPOnly bool `json:"httpOnly,omitempty"`
	// Secure is set to true if the cookie was transmitted over SSL, false
	// otherwise.
	Secure bool `json:"secure,omitempty"`
}

// Header is an HTTP request or response header.
type Header struct {
	// Name is the header name.
	Name string `json:"name"`
	// Value is the header value.
	Value string `json:"value"`
}

// QueryString is a query string parameter on a request.
type QueryString struct {
	// Name is the query parameter name.
	Name string `json:"name"`
	// Value is the query parameter value.
	Value string `json:"value"`
}

// PostData describes posted data on a request.
type PostData struct {
	// MimeType is the MIME type of the posted data.
	MimeType string `json:"mimeType"`
	// Params is a list of posted parameters (in case of URL encoded parameters).
	Params []Param `json:"params"`
	// Text contains the plain text posted data.
	Text string `json:"text"`
}

// Param describes an individual posted parameter.
type Param struct {
	// Name of the posted parameter.
	Name string `json:"name"`
	// Value of the posted parameter.
	Value string `json:"value,omitempty"`
	// Filename of a posted file.
	Filename string `json:"fileName,omitempty"`
	// ContentType is the content type of a posted file.
	ContentType string `json:"contentType,omitempty"`
}

// Content describes details about response content.
type Content struct {
	// Size is the length of the returned content in bytes. Should be equal to
	// response.bodySize if there is no compression and bigger when the content
	// has been compressed.
	Size int64 `json:"size"`
	// MimeType is the MIME type of the response text (value of the Content-Type
	// response header).
	MimeType string `json:"mimeType"`
	// Text contains the response body sent from the server or loaded from the
	// browser cache. This field is populated with textual content only. The text
	// field is either HTTP decoded text or a encoded (e.g. "base64")
	// representation of the response body. Leave out this field if the
	// information is not available.
	Text string `json:"text,omitempty"`
	// Encoding used for response text field e.g "base64". Leave out this field
	// if the text field is HTTP decoded (decompressed & unchunked), than
	// trans-coded from its original character set into UTF-8.
	Encoding string `json:"encoding,omitempty"`
}
