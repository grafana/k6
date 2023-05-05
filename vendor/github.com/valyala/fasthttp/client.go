package fasthttp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Do performs the given http request and fills the given http response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func Do(req *Request, resp *Response) error {
	return defaultClient.Do(req, resp)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
//
// Warning: DoTimeout does not terminate the request itself. The request will
// continue in the background and the response will be discarded.
// If requests take too long and the connection pool gets filled up please
// try using a Client and setting a ReadTimeout.
func DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	return defaultClient.DoTimeout(req, resp, timeout)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return defaultClient.DoDeadline(req, resp, deadline)
}

// DoRedirects performs the given http request and fills the given http response,
// following up to maxRedirectsCount redirects. When the redirect count exceeds
// maxRedirectsCount, ErrTooManyRedirects is returned.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func DoRedirects(req *Request, resp *Response, maxRedirectsCount int) error {
	_, _, err := doRequestFollowRedirects(req, resp, req.URI().String(), maxRedirectsCount, &defaultClient)
	return err
}

// Get returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
func Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return defaultClient.Get(dst, url)
}

// GetTimeout returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return defaultClient.GetTimeout(dst, url, timeout)
}

// GetDeadline returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return defaultClient.GetDeadline(dst, url, deadline)
}

// Post sends POST request to the given url with the given POST arguments.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// Empty POST body is sent if postArgs is nil.
func Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return defaultClient.Post(dst, url, postArgs)
}

var defaultClient Client

// Client implements http client.
//
// Copying Client by value is prohibited. Create new instance instead.
//
// It is safe calling Client methods from concurrently running goroutines.
//
// The fields of a Client should not be changed while it is in use.
type Client struct {
	noCopy noCopy

	// Client name. Used in User-Agent request header.
	//
	// Default client name is used if not set.
	Name string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Callback for establishing new connections to hosts.
	//
	// Default Dial is used if not set.
	Dial DialFunc

	// Attempt to connect to both ipv4 and ipv6 addresses if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// TLS config for https connections.
	//
	// Default TLS config is used if not set.
	TLSConfig *tls.Config

	// Maximum number of connections per each host which may be established.
	//
	// DefaultMaxConnsPerHost is used if not set.
	MaxConnsPerHost int

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Keep-alive connections are closed after this duration.
	//
	// By default connection duration is unlimited.
	MaxConnDuration time.Duration

	// Maximum number of attempts for idempotent calls
	//
	// DefaultMaxIdemponentCallAttempts is used if not set.
	MaxIdemponentCallAttempts int

	// Per-connection buffer size for responses' reading.
	// This also limits the maximum header size.
	//
	// Default buffer size is used if 0.
	ReadBufferSize int

	// Per-connection buffer size for requests' writing.
	//
	// Default buffer size is used if 0.
	WriteBufferSize int

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns ErrBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	// Maximum duration for waiting for a free connection.
	//
	// By default will not waiting, return ErrNoFreeConns immediately
	MaxConnWaitTimeout time.Duration

	// RetryIf controls whether a retry should be attempted after an error.
	//
	// By default will use isIdempotent function
	RetryIf RetryIfFunc

	// Connection pool strategy. Can be either LIFO or FIFO (default).
	ConnPoolStrategy ConnPoolStrategyType

	// StreamResponseBody enables response body streaming
	StreamResponseBody bool

	// ConfigureClient configures the fasthttp.HostClient.
	ConfigureClient func(hc *HostClient) error

	mLock      sync.Mutex
	m          map[string]*HostClient
	ms         map[string]*HostClient
	readerPool sync.Pool
	writerPool sync.Pool
}

// Get returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
func (c *Client) Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return clientGetURL(dst, url, c)
}

// GetTimeout returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func (c *Client) GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return clientGetURLTimeout(dst, url, timeout, c)
}

// GetDeadline returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func (c *Client) GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return clientGetURLDeadline(dst, url, deadline, c)
}

// Post sends POST request to the given url with the given POST arguments.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// Empty POST body is sent if postArgs is nil.
func (c *Client) Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return clientPostURL(dst, url, postArgs, c)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
// Immediately returns ErrTimeout if timeout value is negative.
//
// ErrNoFreeConns is returned if all Client.MaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
//
// Warning: DoTimeout does not terminate the request itself. The request will
// continue in the background and the response will be discarded.
// If requests take too long and the connection pool gets filled up please
// try setting a ReadTimeout.
func (c *Client) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	req.timeout = timeout
	if req.timeout < 0 {
		return ErrTimeout
	}
	return c.Do(req, resp)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
// Immediately returns ErrTimeout if the deadline has already been reached.
//
// ErrNoFreeConns is returned if all Client.MaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	req.timeout = time.Until(deadline)
	if req.timeout < 0 {
		return ErrTimeout
	}
	return c.Do(req, resp)
}

// DoRedirects performs the given http request and fills the given http response,
// following up to maxRedirectsCount redirects. When the redirect count exceeds
// maxRedirectsCount, ErrTooManyRedirects is returned.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) DoRedirects(req *Request, resp *Response, maxRedirectsCount int) error {
	_, _, err := doRequestFollowRedirects(req, resp, req.URI().String(), maxRedirectsCount, c)
	return err
}

// Do performs the given http request and fills the given http response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// Response is ignored if resp is nil.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// ErrNoFreeConns is returned if all Client.MaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) Do(req *Request, resp *Response) error {
	uri := req.URI()
	if uri == nil {
		return ErrorInvalidURI
	}

	host := uri.Host()

	isTLS := false
	if uri.isHTTPS() {
		isTLS = true
	} else if !uri.isHTTP() {
		return fmt.Errorf("unsupported protocol %q. http and https are supported", uri.Scheme())
	}

	startCleaner := false

	c.mLock.Lock()
	m := c.m
	if isTLS {
		m = c.ms
	}
	if m == nil {
		m = make(map[string]*HostClient)
		if isTLS {
			c.ms = m
		} else {
			c.m = m
		}
	}
	hc := m[string(host)]
	if hc == nil {
		hc = &HostClient{
			Addr:                          AddMissingPort(string(host), isTLS),
			Name:                          c.Name,
			NoDefaultUserAgentHeader:      c.NoDefaultUserAgentHeader,
			Dial:                          c.Dial,
			DialDualStack:                 c.DialDualStack,
			IsTLS:                         isTLS,
			TLSConfig:                     c.TLSConfig,
			MaxConns:                      c.MaxConnsPerHost,
			MaxIdleConnDuration:           c.MaxIdleConnDuration,
			MaxConnDuration:               c.MaxConnDuration,
			MaxIdemponentCallAttempts:     c.MaxIdemponentCallAttempts,
			ReadBufferSize:                c.ReadBufferSize,
			WriteBufferSize:               c.WriteBufferSize,
			ReadTimeout:                   c.ReadTimeout,
			WriteTimeout:                  c.WriteTimeout,
			MaxResponseBodySize:           c.MaxResponseBodySize,
			DisableHeaderNamesNormalizing: c.DisableHeaderNamesNormalizing,
			DisablePathNormalizing:        c.DisablePathNormalizing,
			MaxConnWaitTimeout:            c.MaxConnWaitTimeout,
			RetryIf:                       c.RetryIf,
			ConnPoolStrategy:              c.ConnPoolStrategy,
			StreamResponseBody:            c.StreamResponseBody,
			clientReaderPool:              &c.readerPool,
			clientWriterPool:              &c.writerPool,
		}

		if c.ConfigureClient != nil {
			if err := c.ConfigureClient(hc); err != nil {
				c.mLock.Unlock()
				return err
			}
		}

		m[string(host)] = hc
		if len(m) == 1 {
			startCleaner = true
		}
	}

	atomic.AddInt32(&hc.pendingClientRequests, 1)
	defer atomic.AddInt32(&hc.pendingClientRequests, -1)

	c.mLock.Unlock()

	if startCleaner {
		go c.mCleaner(m)
	}

	return hc.Do(req, resp)
}

// CloseIdleConnections closes any connections which were previously
// connected from previous requests but are now sitting idle in a
// "keep-alive" state. It does not interrupt any connections currently
// in use.
func (c *Client) CloseIdleConnections() {
	c.mLock.Lock()
	for _, v := range c.m {
		v.CloseIdleConnections()
	}
	for _, v := range c.ms {
		v.CloseIdleConnections()
	}
	c.mLock.Unlock()
}

func (c *Client) mCleaner(m map[string]*HostClient) {
	mustStop := false

	sleep := c.MaxIdleConnDuration
	if sleep < time.Second {
		sleep = time.Second
	} else if sleep > 10*time.Second {
		sleep = 10 * time.Second
	}

	for {
		time.Sleep(sleep)
		c.mLock.Lock()
		for k, v := range m {
			v.connsLock.Lock()
			if v.connsCount == 0 && atomic.LoadInt32(&v.pendingClientRequests) == 0 {
				delete(m, k)
			}
			v.connsLock.Unlock()
		}
		if len(m) == 0 {
			mustStop = true
		}
		c.mLock.Unlock()

		if mustStop {
			break
		}
	}
}

// DefaultMaxConnsPerHost is the maximum number of concurrent connections
// http client may establish per host by default (i.e. if
// Client.MaxConnsPerHost isn't set).
const DefaultMaxConnsPerHost = 512

// DefaultMaxIdleConnDuration is the default duration before idle keep-alive
// connection is closed.
const DefaultMaxIdleConnDuration = 10 * time.Second

// DefaultMaxIdemponentCallAttempts is the default idempotent calls attempts count.
const DefaultMaxIdemponentCallAttempts = 5

// DialFunc must establish connection to addr.
//
// There is no need in establishing TLS (SSL) connection for https.
// The client automatically converts connection to TLS
// if HostClient.IsTLS is set.
//
// TCP address passed to DialFunc always contains host and port.
// Example TCP addr values:
//
//   - foobar.com:80
//   - foobar.com:443
//   - foobar.com:8080
type DialFunc func(addr string) (net.Conn, error)

// RetryIfFunc signature of retry if function
//
// Request argument passed to RetryIfFunc, if there are any request errors.
type RetryIfFunc func(request *Request) bool

// TransportFunc wraps every request/response.
type TransportFunc func(*Request, *Response) error

// ConnPoolStrategyType define strategy of connection pool enqueue/dequeue
type ConnPoolStrategyType int

const (
	FIFO ConnPoolStrategyType = iota
	LIFO
)

// HostClient balances http requests among hosts listed in Addr.
//
// HostClient may be used for balancing load among multiple upstream hosts.
// While multiple addresses passed to HostClient.Addr may be used for balancing
// load among them, it would be better using LBClient instead, since HostClient
// may unevenly balance load among upstream hosts.
//
// It is forbidden copying HostClient instances. Create new instances instead.
//
// It is safe calling HostClient methods from concurrently running goroutines.
type HostClient struct {
	noCopy noCopy

	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dial in a round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Addr string

	// Client name. Used in User-Agent request header.
	Name string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// Callback for establishing new connection to the host.
	//
	// Default Dial is used if not set.
	Dial DialFunc

	// Attempt to connect to both ipv4 and ipv6 host addresses
	// if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	IsTLS bool

	// Optional TLS config.
	TLSConfig *tls.Config

	// Maximum number of connections which may be established to all hosts
	// listed in Addr.
	//
	// You can change this value while the HostClient is being used
	// with HostClient.SetMaxConns(value)
	//
	// DefaultMaxConnsPerHost is used if not set.
	MaxConns int

	// Keep-alive connections are closed after this duration.
	//
	// By default connection duration is unlimited.
	MaxConnDuration time.Duration

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Maximum number of attempts for idempotent calls
	//
	// DefaultMaxIdemponentCallAttempts is used if not set.
	MaxIdemponentCallAttempts int

	// Per-connection buffer size for responses' reading.
	// This also limits the maximum header size.
	//
	// Default buffer size is used if 0.
	ReadBufferSize int

	// Per-connection buffer size for requests' writing.
	//
	// Default buffer size is used if 0.
	WriteBufferSize int

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns ErrBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	// Will not log potentially sensitive content in error logs
	//
	// This option is useful for servers that handle sensitive data
	// in the request/response.
	//
	// Client logs full errors by default.
	SecureErrorLogMessage bool

	// Maximum duration for waiting for a free connection.
	//
	// By default will not waiting, return ErrNoFreeConns immediately
	MaxConnWaitTimeout time.Duration

	// RetryIf controls whether a retry should be attempted after an error.
	//
	// By default will use isIdempotent function
	RetryIf RetryIfFunc

	// Transport defines a transport-like mechanism that wraps every request/response.
	Transport TransportFunc

	// Connection pool strategy. Can be either LIFO or FIFO (default).
	ConnPoolStrategy ConnPoolStrategyType

	// StreamResponseBody enables response body streaming
	StreamResponseBody bool

	lastUseTime uint32

	connsLock  sync.Mutex
	connsCount int
	conns      []*clientConn
	connsWait  *wantConnQueue

	addrsLock sync.Mutex
	addrs     []string
	addrIdx   uint32

	tlsConfigMap     map[string]*tls.Config
	tlsConfigMapLock sync.Mutex

	readerPool sync.Pool
	writerPool sync.Pool

	clientReaderPool *sync.Pool
	clientWriterPool *sync.Pool

	pendingRequests int32

	// pendingClientRequests counts the number of requests that a Client is currently running using this HostClient.
	// It will be incremented earlier than pendingRequests and will be used by Client to see if the HostClient is still in use.
	pendingClientRequests int32

	connsCleanerRun bool
}

type clientConn struct {
	c net.Conn

	createdTime time.Time
	lastUseTime time.Time
}

var startTimeUnix = time.Now().Unix()

// LastUseTime returns time the client was last used
func (c *HostClient) LastUseTime() time.Time {
	n := atomic.LoadUint32(&c.lastUseTime)
	return time.Unix(startTimeUnix+int64(n), 0)
}

// Get returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
func (c *HostClient) Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return clientGetURL(dst, url, c)
}

// GetTimeout returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func (c *HostClient) GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return clientGetURLTimeout(dst, url, timeout, c)
}

// GetDeadline returns the status code and body of url.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func (c *HostClient) GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return clientGetURLDeadline(dst, url, deadline, c)
}

// Post sends POST request to the given url with the given POST arguments.
//
// The contents of dst will be replaced by the body and returned, if the dst
// is too small a new slice will be allocated.
//
// The function follows redirects. Use Do* for manually handling redirects.
//
// Empty POST body is sent if postArgs is nil.
func (c *HostClient) Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return clientPostURL(dst, url, postArgs, c)
}

type clientDoer interface {
	Do(req *Request, resp *Response) error
}

func clientGetURL(dst []byte, url string, c clientDoer) (statusCode int, body []byte, err error) {
	req := AcquireRequest()

	statusCode, body, err = doRequestFollowRedirectsBuffer(req, dst, url, c)

	ReleaseRequest(req)
	return statusCode, body, err
}

func clientGetURLTimeout(dst []byte, url string, timeout time.Duration, c clientDoer) (statusCode int, body []byte, err error) {
	deadline := time.Now().Add(timeout)
	return clientGetURLDeadline(dst, url, deadline, c)
}

type clientURLResponse struct {
	statusCode int
	body       []byte
	err        error
}

func clientGetURLDeadline(dst []byte, url string, deadline time.Time, c clientDoer) (statusCode int, body []byte, err error) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return 0, dst, ErrTimeout
	}

	var ch chan clientURLResponse
	chv := clientURLResponseChPool.Get()
	if chv == nil {
		chv = make(chan clientURLResponse, 1)
	}
	ch = chv.(chan clientURLResponse)

	// Note that the request continues execution on ErrTimeout until
	// client-specific ReadTimeout exceeds. This helps limiting load
	// on slow hosts by MaxConns* concurrent requests.
	//
	// Without this 'hack' the load on slow host could exceed MaxConns*
	// concurrent requests, since timed out requests on client side
	// usually continue execution on the host.

	var mu sync.Mutex
	var timedout, responded bool

	go func() {
		req := AcquireRequest()

		statusCodeCopy, bodyCopy, errCopy := doRequestFollowRedirectsBuffer(req, dst, url, c)
		mu.Lock()
		{
			if !timedout {
				ch <- clientURLResponse{
					statusCode: statusCodeCopy,
					body:       bodyCopy,
					err:        errCopy,
				}
				responded = true
			}
		}
		mu.Unlock()

		ReleaseRequest(req)
	}()

	tc := AcquireTimer(timeout)
	select {
	case resp := <-ch:
		statusCode = resp.statusCode
		body = resp.body
		err = resp.err
	case <-tc.C:
		mu.Lock()
		{
			if responded {
				resp := <-ch
				statusCode = resp.statusCode
				body = resp.body
				err = resp.err
			} else {
				timedout = true
				err = ErrTimeout
				body = dst
			}
		}
		mu.Unlock()
	}
	ReleaseTimer(tc)

	clientURLResponseChPool.Put(chv)

	return statusCode, body, err
}

var clientURLResponseChPool sync.Pool

func clientPostURL(dst []byte, url string, postArgs *Args, c clientDoer) (statusCode int, body []byte, err error) {
	req := AcquireRequest()
	defer ReleaseRequest(req)

	req.Header.SetMethod(MethodPost)
	req.Header.SetContentTypeBytes(strPostArgsContentType)
	if postArgs != nil {
		if _, err := postArgs.WriteTo(req.BodyWriter()); err != nil {
			return 0, nil, err
		}
	}

	statusCode, body, err = doRequestFollowRedirectsBuffer(req, dst, url, c)

	return statusCode, body, err
}

var (
	// ErrMissingLocation is returned by clients when the Location header is missing on
	// an HTTP response with a redirect status code.
	ErrMissingLocation = errors.New("missing Location header for http redirect")
	// ErrTooManyRedirects is returned by clients when the number of redirects followed
	// exceed the max count.
	ErrTooManyRedirects = errors.New("too many redirects detected when doing the request")

	// HostClients are only able to follow redirects to the same protocol.
	ErrHostClientRedirectToDifferentScheme = errors.New("HostClient can't follow redirects to a different protocol, please use Client instead")
)

const defaultMaxRedirectsCount = 16

func doRequestFollowRedirectsBuffer(req *Request, dst []byte, url string, c clientDoer) (statusCode int, body []byte, err error) {
	resp := AcquireResponse()
	bodyBuf := resp.bodyBuffer()
	resp.keepBodyBuffer = true
	oldBody := bodyBuf.B
	bodyBuf.B = dst

	statusCode, _, err = doRequestFollowRedirects(req, resp, url, defaultMaxRedirectsCount, c)

	body = bodyBuf.B
	bodyBuf.B = oldBody
	resp.keepBodyBuffer = false
	ReleaseResponse(resp)

	return statusCode, body, err
}

func doRequestFollowRedirects(req *Request, resp *Response, url string, maxRedirectsCount int, c clientDoer) (statusCode int, body []byte, err error) {
	redirectsCount := 0

	for {
		req.SetRequestURI(url)
		if err := req.parseURI(); err != nil {
			return 0, nil, err
		}

		if err = c.Do(req, resp); err != nil {
			break
		}
		statusCode = resp.Header.StatusCode()
		if !StatusCodeIsRedirect(statusCode) {
			break
		}

		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = ErrTooManyRedirects
			break
		}
		location := resp.Header.peek(strLocation)
		if len(location) == 0 {
			err = ErrMissingLocation
			break
		}
		url = getRedirectURL(url, location)
	}

	return statusCode, body, err
}

func getRedirectURL(baseURL string, location []byte) string {
	u := AcquireURI()
	u.Update(baseURL)
	u.UpdateBytes(location)
	redirectURL := u.String()
	ReleaseURI(u)
	return redirectURL
}

// StatusCodeIsRedirect returns true if the status code indicates a redirect.
func StatusCodeIsRedirect(statusCode int) bool {
	return statusCode == StatusMovedPermanently ||
		statusCode == StatusFound ||
		statusCode == StatusSeeOther ||
		statusCode == StatusTemporaryRedirect ||
		statusCode == StatusPermanentRedirect
}

var (
	requestPool  sync.Pool
	responsePool sync.Pool
)

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse return resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its' members after returning
// it to response pool.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
// Immediately returns ErrTimeout if timeout value is negative.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
//
// Warning: DoTimeout does not terminate the request itself. The request will
// continue in the background and the response will be discarded.
// If requests take too long and the connection pool gets filled up please
// try setting a ReadTimeout.
func (c *HostClient) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	req.timeout = timeout
	if req.timeout < 0 {
		return ErrTimeout
	}
	return c.Do(req, resp)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
// Immediately returns ErrTimeout if the deadline has already been reached.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	req.timeout = time.Until(deadline)
	if req.timeout < 0 {
		return ErrTimeout
	}
	return c.Do(req, resp)
}

// DoRedirects performs the given http request and fills the given http response,
// following up to maxRedirectsCount redirects. When the redirect count exceeds
// maxRedirectsCount, ErrTooManyRedirects is returned.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoRedirects(req *Request, resp *Response, maxRedirectsCount int) error {
	_, _, err := doRequestFollowRedirects(req, resp, req.URI().String(), maxRedirectsCount, c)
	return err
}

// Do performs the given http request and sets the corresponding response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) Do(req *Request, resp *Response) error {
	var err error
	var retry bool
	maxAttempts := c.MaxIdemponentCallAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxIdemponentCallAttempts
	}
	isRequestRetryable := isIdempotent
	if c.RetryIf != nil {
		isRequestRetryable = c.RetryIf
	}
	attempts := 0
	hasBodyStream := req.IsBodyStream()

	atomic.AddInt32(&c.pendingRequests, 1)
	for {
		retry, err = c.do(req, resp)
		if err == nil || !retry {
			break
		}

		if hasBodyStream {
			break
		}
		if !isRequestRetryable(req) {
			// Retry non-idempotent requests if the server closes
			// the connection before sending the response.
			//
			// This case is possible if the server closes the idle
			// keep-alive connection on timeout.
			//
			// Apache and nginx usually do this.
			if err != io.EOF {
				break
			}
		}
		attempts++
		if attempts >= maxAttempts {
			break
		}
	}
	atomic.AddInt32(&c.pendingRequests, -1)

	if err == io.EOF {
		err = ErrConnectionClosed
	}
	return err
}

// PendingRequests returns the current number of requests the client
// is executing.
//
// This function may be used for balancing load among multiple HostClient
// instances.
func (c *HostClient) PendingRequests() int {
	return int(atomic.LoadInt32(&c.pendingRequests))
}

func isIdempotent(req *Request) bool {
	return req.Header.IsGet() || req.Header.IsHead() || req.Header.IsPut()
}

func (c *HostClient) do(req *Request, resp *Response) (bool, error) {
	if resp == nil {
		resp = AcquireResponse()
		defer ReleaseResponse(resp)
	}

	ok, err := c.doNonNilReqResp(req, resp)

	return ok, err
}

func (c *HostClient) doNonNilReqResp(req *Request, resp *Response) (bool, error) {
	if req == nil {
		// for debugging purposes
		panic("BUG: req cannot be nil")
	}
	if resp == nil {
		// for debugging purposes
		panic("BUG: resp cannot be nil")
	}

	// Secure header error logs configuration
	resp.secureErrorLogMessage = c.SecureErrorLogMessage
	resp.Header.secureErrorLogMessage = c.SecureErrorLogMessage
	req.secureErrorLogMessage = c.SecureErrorLogMessage
	req.Header.secureErrorLogMessage = c.SecureErrorLogMessage

	if c.IsTLS != req.URI().isHTTPS() {
		return false, ErrHostClientRedirectToDifferentScheme
	}

	atomic.StoreUint32(&c.lastUseTime, uint32(time.Now().Unix()-startTimeUnix))

	// Free up resources occupied by response before sending the request,
	// so the GC may reclaim these resources (e.g. response body).

	// backing up SkipBody in case it was set explicitly
	customSkipBody := resp.SkipBody
	customStreamBody := resp.StreamBody || c.StreamResponseBody
	resp.Reset()
	resp.SkipBody = customSkipBody
	resp.StreamBody = customStreamBody

	req.URI().DisablePathNormalizing = c.DisablePathNormalizing

	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		userAgent := c.Name
		if userAgent == "" && !c.NoDefaultUserAgentHeader {
			userAgent = defaultUserAgent
		}
		if userAgent != "" {
			req.Header.userAgent = append(req.Header.userAgent[:], userAgent...)
		}
	}
	if c.Transport != nil {
		err := c.Transport(req, resp)
		return err == nil, err
	}

	var deadline time.Time
	if req.timeout > 0 {
		deadline = time.Now().Add(req.timeout)
	}

	cc, err := c.acquireConn(req.timeout, req.ConnectionClose())
	if err != nil {
		return false, err
	}
	conn := cc.c

	resp.parseNetConn(conn)

	writeDeadline := deadline
	if c.WriteTimeout > 0 {
		tmpWriteDeadline := time.Now().Add(c.WriteTimeout)
		if writeDeadline.IsZero() || tmpWriteDeadline.Before(writeDeadline) {
			writeDeadline = tmpWriteDeadline
		}
	}
	if !writeDeadline.IsZero() {
		// Set Deadline every time, since golang has fixed the performance issue
		// See https://github.com/golang/go/issues/15133#issuecomment-271571395 for details
		if err = conn.SetWriteDeadline(writeDeadline); err != nil {
			c.closeConn(cc)
			return true, err
		}
	}

	resetConnection := false
	if c.MaxConnDuration > 0 && time.Since(cc.createdTime) > c.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	bw := c.acquireWriter(conn)
	err = req.Write(bw)

	if resetConnection {
		req.Header.ResetConnectionClose()
	}

	if err == nil {
		err = bw.Flush()
	}
	c.releaseWriter(bw)

	// Return ErrTimeout on any timeout.
	if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
		err = ErrTimeout
	}

	isConnRST := isConnectionReset(err)
	if err != nil && !isConnRST {
		c.closeConn(cc)
		return true, err
	}

	readDeadline := deadline
	if c.ReadTimeout > 0 {
		tmpReadDeadline := time.Now().Add(c.ReadTimeout)
		if readDeadline.IsZero() || tmpReadDeadline.Before(readDeadline) {
			readDeadline = tmpReadDeadline
		}
	}
	if !readDeadline.IsZero() {
		// Set Deadline every time, since golang has fixed the performance issue
		// See https://github.com/golang/go/issues/15133#issuecomment-271571395 for details
		if err = conn.SetReadDeadline(readDeadline); err != nil {
			c.closeConn(cc)
			return true, err
		}
	}

	if customSkipBody || req.Header.IsHead() {
		resp.SkipBody = true
	}
	if c.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}

	br := c.acquireReader(conn)
	err = resp.ReadLimitBody(br, c.MaxResponseBodySize)
	c.releaseReader(br)
	if err != nil {
		c.closeConn(cc)
		// Don't retry in case of ErrBodyTooLarge since we will just get the same again.
		retry := err != ErrBodyTooLarge
		return retry, err
	}

	closeConn := resetConnection || req.ConnectionClose() || resp.ConnectionClose() || isConnRST
	if customStreamBody && resp.bodyStream != nil {
		rbs := resp.bodyStream
		resp.bodyStream = newCloseReader(rbs, func() error {
			if r, ok := rbs.(*requestStream); ok {
				releaseRequestStream(r)
			}
			if closeConn {
				c.closeConn(cc)
			} else {
				c.releaseConn(cc)
			}
			return nil
		})
		return false, nil
	}

	if closeConn {
		c.closeConn(cc)
	} else {
		c.releaseConn(cc)
	}
	return false, nil
}

var (
	// ErrNoFreeConns is returned when no free connections available
	// to the given host.
	//
	// Increase the allowed number of connections per host if you
	// see this error.
	ErrNoFreeConns = errors.New("no free connections available to host")

	// ErrConnectionClosed may be returned from client methods if the server
	// closes connection before returning the first response byte.
	//
	// If you see this error, then either fix the server by returning
	// 'Connection: close' response header before closing the connection
	// or add 'Connection: close' request header before sending requests
	// to broken server.
	ErrConnectionClosed = errors.New("the server closed connection before returning the first response byte. " +
		"Make sure the server returns 'Connection: close' response header before closing the connection")

	// ErrConnPoolStrategyNotImpl is returned when HostClient.ConnPoolStrategy is not implement yet.
	// If you see this error, then you need to check your HostClient configuration.
	ErrConnPoolStrategyNotImpl = errors.New("connection pool strategy is not implement")
)

type timeoutError struct{}

func (e *timeoutError) Error() string {
	return "timeout"
}

// Only implement the Timeout() function of the net.Error interface.
// This allows for checks like:
//
//	if x, ok := err.(interface{ Timeout() bool }); ok && x.Timeout() {
func (e *timeoutError) Timeout() bool {
	return true
}

// ErrTimeout is returned from timed out calls.
var ErrTimeout = &timeoutError{}

// SetMaxConns sets up the maximum number of connections which may be established to all hosts listed in Addr.
func (c *HostClient) SetMaxConns(newMaxConns int) {
	c.connsLock.Lock()
	c.MaxConns = newMaxConns
	c.connsLock.Unlock()
}

func (c *HostClient) acquireConn(reqTimeout time.Duration, connectionClose bool) (cc *clientConn, err error) {
	createConn := false
	startCleaner := false

	var n int
	c.connsLock.Lock()
	n = len(c.conns)
	if n == 0 {
		maxConns := c.MaxConns
		if maxConns <= 0 {
			maxConns = DefaultMaxConnsPerHost
		}
		if c.connsCount < maxConns {
			c.connsCount++
			createConn = true
			if !c.connsCleanerRun && !connectionClose {
				startCleaner = true
				c.connsCleanerRun = true
			}
		}
	} else {
		switch c.ConnPoolStrategy {
		case LIFO:
			n--
			cc = c.conns[n]
			c.conns[n] = nil
			c.conns = c.conns[:n]
		case FIFO:
			cc = c.conns[0]
			copy(c.conns, c.conns[1:])
			c.conns[n-1] = nil
			c.conns = c.conns[:n-1]
		default:
			c.connsLock.Unlock()
			return nil, ErrConnPoolStrategyNotImpl
		}
	}
	c.connsLock.Unlock()

	if cc != nil {
		return cc, nil
	}
	if !createConn {
		if c.MaxConnWaitTimeout <= 0 {
			return nil, ErrNoFreeConns
		}

		// reqTimeout    c.MaxConnWaitTimeout   wait duration
		//     d1                 d2            min(d1, d2)
		//  0(not set)            d2            d2
		//     d1            0(don't wait)      0(don't wait)
		//  0(not set)            d2            d2
		timeout := c.MaxConnWaitTimeout
		timeoutOverridden := false
		// reqTimeout == 0 means not set
		if reqTimeout > 0 && reqTimeout < timeout {
			timeout = reqTimeout
			timeoutOverridden = true
		}

		// wait for a free connection
		tc := AcquireTimer(timeout)
		defer ReleaseTimer(tc)

		w := &wantConn{
			ready: make(chan struct{}, 1),
		}
		defer func() {
			if err != nil {
				w.cancel(c, err)
			}
		}()

		c.queueForIdle(w)

		select {
		case <-w.ready:
			return w.conn, w.err
		case <-tc.C:
			if timeoutOverridden {
				return nil, ErrTimeout
			}
			return nil, ErrNoFreeConns
		}
	}

	if startCleaner {
		go c.connsCleaner()
	}

	conn, err := c.dialHostHard(reqTimeout)
	if err != nil {
		c.decConnsCount()
		return nil, err
	}
	cc = acquireClientConn(conn)

	return cc, nil
}

func (c *HostClient) queueForIdle(w *wantConn) {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	if c.connsWait == nil {
		c.connsWait = &wantConnQueue{}
	}
	c.connsWait.clearFront()
	c.connsWait.pushBack(w)
}

func (c *HostClient) dialConnFor(w *wantConn) {
	conn, err := c.dialHostHard(0)
	if err != nil {
		w.tryDeliver(nil, err)
		c.decConnsCount()
		return
	}

	cc := acquireClientConn(conn)
	if !w.tryDeliver(cc, nil) {
		// not delivered, return idle connection
		c.releaseConn(cc)
	}
}

// CloseIdleConnections closes any connections which were previously
// connected from previous requests but are now sitting idle in a
// "keep-alive" state. It does not interrupt any connections currently
// in use.
func (c *HostClient) CloseIdleConnections() {
	c.connsLock.Lock()
	scratch := append([]*clientConn{}, c.conns...)
	for i := range c.conns {
		c.conns[i] = nil
	}
	c.conns = c.conns[:0]
	c.connsLock.Unlock()

	for _, cc := range scratch {
		c.closeConn(cc)
	}
}

func (c *HostClient) connsCleaner() {
	var (
		scratch             []*clientConn
		maxIdleConnDuration = c.MaxIdleConnDuration
	)
	if maxIdleConnDuration <= 0 {
		maxIdleConnDuration = DefaultMaxIdleConnDuration
	}
	for {
		currentTime := time.Now()

		// Determine idle connections to be closed.
		c.connsLock.Lock()
		conns := c.conns
		n := len(conns)
		i := 0
		for i < n && currentTime.Sub(conns[i].lastUseTime) > maxIdleConnDuration {
			i++
		}
		sleepFor := maxIdleConnDuration
		if i < n {
			// + 1 so we actually sleep past the expiration time and not up to it.
			// Otherwise the > check above would still fail.
			sleepFor = maxIdleConnDuration - currentTime.Sub(conns[i].lastUseTime) + 1
		}
		scratch = append(scratch[:0], conns[:i]...)
		if i > 0 {
			m := copy(conns, conns[i:])
			for i = m; i < n; i++ {
				conns[i] = nil
			}
			c.conns = conns[:m]
		}
		c.connsLock.Unlock()

		// Close idle connections.
		for i, cc := range scratch {
			c.closeConn(cc)
			scratch[i] = nil
		}

		// Determine whether to stop the connsCleaner.
		c.connsLock.Lock()
		mustStop := c.connsCount == 0
		if mustStop {
			c.connsCleanerRun = false
		}
		c.connsLock.Unlock()
		if mustStop {
			break
		}

		time.Sleep(sleepFor)
	}
}

func (c *HostClient) closeConn(cc *clientConn) {
	c.decConnsCount()
	cc.c.Close()
	releaseClientConn(cc)
}

func (c *HostClient) decConnsCount() {
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.connsCount--
		c.connsLock.Unlock()
		return
	}

	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	dialed := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				go c.dialConnFor(w)
				dialed = true
				break
			}
		}
	}
	if !dialed {
		c.connsCount--
	}
}

// ConnsCount returns connection count of HostClient
func (c *HostClient) ConnsCount() int {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()

	return c.connsCount
}

func acquireClientConn(conn net.Conn) *clientConn {
	v := clientConnPool.Get()
	if v == nil {
		v = &clientConn{}
	}
	cc := v.(*clientConn)
	cc.c = conn
	cc.createdTime = time.Now()
	return cc
}

func releaseClientConn(cc *clientConn) {
	// Reset all fields.
	*cc = clientConn{}
	clientConnPool.Put(cc)
}

var clientConnPool sync.Pool

func (c *HostClient) releaseConn(cc *clientConn) {
	cc.lastUseTime = time.Now()
	if c.MaxConnWaitTimeout <= 0 {
		c.connsLock.Lock()
		c.conns = append(c.conns, cc)
		c.connsLock.Unlock()
		return
	}

	// try to deliver an idle connection to a *wantConn
	c.connsLock.Lock()
	defer c.connsLock.Unlock()
	delivered := false
	if q := c.connsWait; q != nil && q.len() > 0 {
		for q.len() > 0 {
			w := q.popFront()
			if w.waiting() {
				delivered = w.tryDeliver(cc, nil)
				break
			}
		}
	}
	if !delivered {
		c.conns = append(c.conns, cc)
	}
}

func (c *HostClient) acquireWriter(conn net.Conn) *bufio.Writer {
	var v interface{}
	if c.clientWriterPool != nil {
		v = c.clientWriterPool.Get()
		if v == nil {
			n := c.WriteBufferSize
			if n <= 0 {
				n = defaultWriteBufferSize
			}
			return bufio.NewWriterSize(conn, n)
		}
	} else {
		v = c.writerPool.Get()
		if v == nil {
			n := c.WriteBufferSize
			if n <= 0 {
				n = defaultWriteBufferSize
			}
			return bufio.NewWriterSize(conn, n)
		}
	}

	bw := v.(*bufio.Writer)
	bw.Reset(conn)
	return bw
}

func (c *HostClient) releaseWriter(bw *bufio.Writer) {
	if c.clientWriterPool != nil {
		c.clientWriterPool.Put(bw)
	} else {
		c.writerPool.Put(bw)
	}
}

func (c *HostClient) acquireReader(conn net.Conn) *bufio.Reader {
	var v interface{}
	if c.clientReaderPool != nil {
		v = c.clientReaderPool.Get()
		if v == nil {
			n := c.ReadBufferSize
			if n <= 0 {
				n = defaultReadBufferSize
			}
			return bufio.NewReaderSize(conn, n)
		}
	} else {
		v = c.readerPool.Get()
		if v == nil {
			n := c.ReadBufferSize
			if n <= 0 {
				n = defaultReadBufferSize
			}
			return bufio.NewReaderSize(conn, n)
		}
	}

	br := v.(*bufio.Reader)
	br.Reset(conn)
	return br
}

func (c *HostClient) releaseReader(br *bufio.Reader) {
	if c.clientReaderPool != nil {
		c.clientReaderPool.Put(br)
	} else {
		c.readerPool.Put(br)
	}
}

func newClientTLSConfig(c *tls.Config, addr string) *tls.Config {
	if c == nil {
		c = &tls.Config{}
	} else {
		c = c.Clone()
	}

	if len(c.ServerName) == 0 {
		serverName := tlsServerName(addr)
		if serverName == "*" {
			c.InsecureSkipVerify = true
		} else {
			c.ServerName = serverName
		}
	}
	return c
}

func tlsServerName(addr string) string {
	if !strings.Contains(addr, ":") {
		return addr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "*"
	}
	return host
}

func (c *HostClient) nextAddr() string {
	c.addrsLock.Lock()
	if c.addrs == nil {
		c.addrs = strings.Split(c.Addr, ",")
	}
	addr := c.addrs[0]
	if len(c.addrs) > 1 {
		addr = c.addrs[c.addrIdx%uint32(len(c.addrs))]
		c.addrIdx++
	}
	c.addrsLock.Unlock()
	return addr
}

func (c *HostClient) dialHostHard(dialTimeout time.Duration) (conn net.Conn, err error) {
	// use dialTimeout to control the timeout of each dial. It does not work if dialTimeout is 0 or dial has been set.
	// attempt to dial all the available hosts before giving up.

	c.addrsLock.Lock()
	n := len(c.addrs)
	c.addrsLock.Unlock()

	if n == 0 {
		// It looks like c.addrs isn't initialized yet.
		n = 1
	}

	dial := c.Dial
	if dialTimeout != 0 && dial == nil {
		dial = func(addr string) (net.Conn, error) {
			return DialTimeout(addr, dialTimeout)
		}
	}

	timeout := c.ReadTimeout + c.WriteTimeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	deadline := time.Now().Add(timeout)
	for n > 0 {
		addr := c.nextAddr()
		tlsConfig := c.cachedTLSConfig(addr)
		conn, err = dialAddr(addr, dial, c.DialDualStack, c.IsTLS, tlsConfig, c.WriteTimeout)
		if err == nil {
			return conn, nil
		}
		if time.Since(deadline) >= 0 {
			break
		}
		n--
	}
	return nil, err
}

func (c *HostClient) cachedTLSConfig(addr string) *tls.Config {
	if !c.IsTLS {
		return nil
	}

	c.tlsConfigMapLock.Lock()
	if c.tlsConfigMap == nil {
		c.tlsConfigMap = make(map[string]*tls.Config)
	}
	cfg := c.tlsConfigMap[addr]
	if cfg == nil {
		cfg = newClientTLSConfig(c.TLSConfig, addr)
		c.tlsConfigMap[addr] = cfg
	}
	c.tlsConfigMapLock.Unlock()

	return cfg
}

// ErrTLSHandshakeTimeout indicates there is a timeout from tls handshake.
var ErrTLSHandshakeTimeout = errors.New("tls handshake timed out")

func tlsClientHandshake(rawConn net.Conn, tlsConfig *tls.Config, deadline time.Time) (_ net.Conn, retErr error) {
	defer func() {
		if retErr != nil {
			rawConn.Close()
		}
	}()
	conn := tls.Client(rawConn, tlsConfig)
	err := conn.SetDeadline(deadline)
	if err != nil {
		return nil, err
	}
	err = conn.Handshake()
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return nil, ErrTLSHandshakeTimeout
	}
	if err != nil {
		return nil, err
	}
	err = conn.SetDeadline(time.Time{})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func dialAddr(addr string, dial DialFunc, dialDualStack, isTLS bool, tlsConfig *tls.Config, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	if dial == nil {
		if dialDualStack {
			dial = DialDualStack
		} else {
			dial = Dial
		}
		addr = AddMissingPort(addr, isTLS)
	}
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("dialling unsuccessful. Please report this bug!")
	}

	// We assume that any conn that has the Handshake() method is a TLS conn already.
	// This doesn't cover just tls.Conn but also other TLS implementations.
	_, isTLSAlready := conn.(interface{ Handshake() error })

	if isTLS && !isTLSAlready {
		if timeout == 0 {
			return tls.Client(conn, tlsConfig), nil
		}
		return tlsClientHandshake(conn, tlsConfig, deadline)
	}
	return conn, nil
}

// AddMissingPort adds a port to a host if it is missing.
// A literal IPv6 address in hostport must be enclosed in square
// brackets, as in "[::1]:80", "[::1%lo0]:80".
func AddMissingPort(addr string, isTLS bool) string {
	addrLen := len(addr)
	if addrLen == 0 {
		return addr
	}

	isIP6 := addr[0] == '['
	if isIP6 {
		// if the IPv6 has opening bracket but closing bracket is the last char then it doesn't have a port
		isIP6WithoutPort := addr[addrLen-1] == ']'
		if !isIP6WithoutPort {
			return addr
		}
	} else { // IPv4
		columnPos := strings.LastIndexByte(addr, ':')
		if columnPos > 0 {
			return addr
		}
	}
	port := ":80"
	if isTLS {
		port = ":443"
	}
	return addr + port
}

// A wantConn records state about a wanted connection
// (that is, an active call to getConn).
// The conn may be gotten by dialing or by finding an idle connection,
// or a cancellation may make the conn no longer wanted.
// These three options are racing against each other and use
// wantConn to coordinate and agree about the winning outcome.
//
// inspired by net/http/transport.go
type wantConn struct {
	ready chan struct{}
	mu    sync.Mutex // protects conn, err, close(ready)
	conn  *clientConn
	err   error
}

// waiting reports whether w is still waiting for an answer (connection or error).
func (w *wantConn) waiting() bool {
	select {
	case <-w.ready:
		return false
	default:
		return true
	}
}

// tryDeliver attempts to deliver conn, err to w and reports whether it succeeded.
func (w *wantConn) tryDeliver(conn *clientConn, err error) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil || w.err != nil {
		return false
	}
	w.conn = conn
	w.err = err
	if w.conn == nil && w.err == nil {
		panic("fasthttp: internal error: misuse of tryDeliver")
	}
	close(w.ready)
	return true
}

// cancel marks w as no longer wanting a result (for example, due to cancellation).
// If a connection has been delivered already, cancel returns it with c.releaseConn.
func (w *wantConn) cancel(c *HostClient, err error) {
	w.mu.Lock()
	if w.conn == nil && w.err == nil {
		close(w.ready) // catch misbehavior in future delivery
	}

	conn := w.conn
	w.conn = nil
	w.err = err
	w.mu.Unlock()

	if conn != nil {
		c.releaseConn(conn)
	}
}

// A wantConnQueue is a queue of wantConns.
//
// inspired by net/http/transport.go
type wantConnQueue struct {
	// This is a queue, not a deque.
	// It is split into two stages - head[headPos:] and tail.
	// popFront is trivial (headPos++) on the first stage, and
	// pushBack is trivial (append) on the second stage.
	// If the first stage is empty, popFront can swap the
	// first and second stages to remedy the situation.
	//
	// This two-stage split is analogous to the use of two lists
	// in Okasaki's purely functional queue but without the
	// overhead of reversing the list when swapping stages.
	head    []*wantConn
	headPos int
	tail    []*wantConn
}

// len returns the number of items in the queue.
func (q *wantConnQueue) len() int {
	return len(q.head) - q.headPos + len(q.tail)
}

// pushBack adds w to the back of the queue.
func (q *wantConnQueue) pushBack(w *wantConn) {
	q.tail = append(q.tail, w)
}

// popFront removes and returns the wantConn at the front of the queue.
func (q *wantConnQueue) popFront() *wantConn {
	if q.headPos >= len(q.head) {
		if len(q.tail) == 0 {
			return nil
		}
		// Pick up tail as new head, clear tail.
		q.head, q.headPos, q.tail = q.tail, 0, q.head[:0]
	}

	w := q.head[q.headPos]
	q.head[q.headPos] = nil
	q.headPos++
	return w
}

// peekFront returns the wantConn at the front of the queue without removing it.
func (q *wantConnQueue) peekFront() *wantConn {
	if q.headPos < len(q.head) {
		return q.head[q.headPos]
	}
	if len(q.tail) > 0 {
		return q.tail[0]
	}
	return nil
}

// clearFront pops any wantConns that are no longer waiting from the head of the
// queue, reporting whether any were popped.
func (q *wantConnQueue) clearFront() (cleaned bool) {
	for {
		w := q.peekFront()
		if w == nil || w.waiting() {
			return cleaned
		}
		q.popFront()
		cleaned = true
	}
}

// PipelineClient pipelines requests over a limited set of concurrent
// connections to the given Addr.
//
// This client may be used in highly loaded HTTP-based RPC systems for reducing
// context switches and network level overhead.
// See https://en.wikipedia.org/wiki/HTTP_pipelining for details.
//
// It is forbidden copying PipelineClient instances. Create new instances
// instead.
//
// It is safe calling PipelineClient methods from concurrently running
// goroutines.
type PipelineClient struct {
	noCopy noCopy

	// Address of the host to connect to.
	Addr string

	// PipelineClient name. Used in User-Agent request header.
	Name string

	// NoDefaultUserAgentHeader when set to true, causes the default
	// User-Agent header to be excluded from the Request.
	NoDefaultUserAgentHeader bool

	// The maximum number of concurrent connections to the Addr.
	//
	// A single connection is used by default.
	MaxConns int

	// The maximum number of pending pipelined requests over
	// a single connection to Addr.
	//
	// DefaultMaxPendingRequests is used by default.
	MaxPendingRequests int

	// The maximum delay before sending pipelined requests as a batch
	// to the server.
	//
	// By default requests are sent immediately to the server.
	MaxBatchDelay time.Duration

	// Callback for connection establishing to the host.
	//
	// Default Dial is used if not set.
	Dial DialFunc

	// Attempt to connect to both ipv4 and ipv6 host addresses
	// if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// Response header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	IsTLS bool

	// Optional TLS config.
	TLSConfig *tls.Config

	// Idle connection to the host is closed after this duration.
	//
	// By default idle connection is closed after
	// DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Buffer size for responses' reading.
	// This also limits the maximum header size.
	//
	// Default buffer size is used if 0.
	ReadBufferSize int

	// Buffer size for requests' writing.
	//
	// Default buffer size is used if 0.
	WriteBufferSize int

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Logger for logging client errors.
	//
	// By default standard logger from log package is used.
	Logger Logger

	connClients     []*pipelineConnClient
	connClientsLock sync.Mutex
}

type pipelineConnClient struct {
	noCopy noCopy

	Addr                          string
	Name                          string
	NoDefaultUserAgentHeader      bool
	MaxPendingRequests            int
	MaxBatchDelay                 time.Duration
	Dial                          DialFunc
	DialDualStack                 bool
	DisableHeaderNamesNormalizing bool
	DisablePathNormalizing        bool
	IsTLS                         bool
	TLSConfig                     *tls.Config
	MaxIdleConnDuration           time.Duration
	ReadBufferSize                int
	WriteBufferSize               int
	ReadTimeout                   time.Duration
	WriteTimeout                  time.Duration
	Logger                        Logger

	workPool sync.Pool

	chLock sync.Mutex
	chW    chan *pipelineWork
	chR    chan *pipelineWork

	tlsConfigLock sync.Mutex
	tlsConfig     *tls.Config
}

type pipelineWork struct {
	reqCopy  Request
	respCopy Response
	req      *Request
	resp     *Response
	t        *time.Timer
	deadline time.Time
	err      error
	done     chan struct{}
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
//
// Warning: DoTimeout does not terminate the request itself. The request will
// continue in the background and the response will be discarded.
// If requests take too long and the connection pool gets filled up please
// try setting a ReadTimeout.
func (c *PipelineClient) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	return c.DoDeadline(req, resp, time.Now().Add(timeout))
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects.
//
// Response is ignored if resp is nil.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *PipelineClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return c.getConnClient().DoDeadline(req, resp, deadline)
}

func (c *pipelineConnClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	c.init()

	timeout := time.Until(deadline)
	if timeout < 0 {
		return ErrTimeout
	}

	if c.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}

	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		userAgent := c.Name
		if userAgent == "" && !c.NoDefaultUserAgentHeader {
			userAgent = defaultUserAgent
		}
		if userAgent != "" {
			req.Header.userAgent = append(req.Header.userAgent[:], userAgent...)
		}
	}

	w := c.acquirePipelineWork(timeout)
	w.respCopy.Header.disableNormalizing = c.DisableHeaderNamesNormalizing
	w.req = &w.reqCopy
	w.resp = &w.respCopy

	// Make a copy of the request in order to avoid data races on timeouts
	req.copyToSkipBody(&w.reqCopy)
	swapRequestBody(req, &w.reqCopy)

	// Put the request to outgoing queue
	select {
	case c.chW <- w:
		// Fast path: len(c.ch) < cap(c.ch)
	default:
		// Slow path
		select {
		case c.chW <- w:
		case <-w.t.C:
			c.releasePipelineWork(w)
			return ErrTimeout
		}
	}

	// Wait for the response
	var err error
	select {
	case <-w.done:
		if resp != nil {
			w.respCopy.copyToSkipBody(resp)
			swapResponseBody(resp, &w.respCopy)
		}
		err = w.err
		c.releasePipelineWork(w)
	case <-w.t.C:
		err = ErrTimeout
	}

	return err
}

func (c *pipelineConnClient) acquirePipelineWork(timeout time.Duration) (w *pipelineWork) {
	v := c.workPool.Get()
	if v != nil {
		w = v.(*pipelineWork)
	} else {
		w = &pipelineWork{
			done: make(chan struct{}, 1),
		}
	}
	if timeout > 0 {
		if w.t == nil {
			w.t = time.NewTimer(timeout)
		} else {
			w.t.Reset(timeout)
		}
		w.deadline = time.Now().Add(timeout)
	} else {
		w.deadline = zeroTime
	}
	return w
}

func (c *pipelineConnClient) releasePipelineWork(w *pipelineWork) {
	if w.t != nil {
		w.t.Stop()
	}
	w.reqCopy.Reset()
	w.respCopy.Reset()
	w.req = nil
	w.resp = nil
	w.err = nil
	c.workPool.Put(w)
}

// Do performs the given http request and sets the corresponding response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// The function doesn't follow redirects. Use Get* for following redirects.
//
// Response is ignored if resp is nil.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *PipelineClient) Do(req *Request, resp *Response) error {
	return c.getConnClient().Do(req, resp)
}

func (c *pipelineConnClient) Do(req *Request, resp *Response) error {
	c.init()

	if c.DisablePathNormalizing {
		req.URI().DisablePathNormalizing = true
	}

	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		userAgent := c.Name
		if userAgent == "" && !c.NoDefaultUserAgentHeader {
			userAgent = defaultUserAgent
		}
		if userAgent != "" {
			req.Header.userAgent = append(req.Header.userAgent[:], userAgent...)
		}
	}

	w := c.acquirePipelineWork(0)
	w.req = req
	if resp != nil {
		resp.Header.disableNormalizing = c.DisableHeaderNamesNormalizing
		w.resp = resp
	} else {
		w.resp = &w.respCopy
	}

	// Put the request to outgoing queue
	select {
	case c.chW <- w:
	default:
		// Try substituting the oldest w with the current one.
		select {
		case wOld := <-c.chW:
			wOld.err = ErrPipelineOverflow
			wOld.done <- struct{}{}
		default:
		}
		select {
		case c.chW <- w:
		default:
			c.releasePipelineWork(w)
			return ErrPipelineOverflow
		}
	}

	// Wait for the response
	<-w.done
	err := w.err

	c.releasePipelineWork(w)

	return err
}

func (c *PipelineClient) getConnClient() *pipelineConnClient {
	c.connClientsLock.Lock()
	cc := c.getConnClientUnlocked()
	c.connClientsLock.Unlock()
	return cc
}

func (c *PipelineClient) getConnClientUnlocked() *pipelineConnClient {
	if len(c.connClients) == 0 {
		return c.newConnClient()
	}

	// Return the client with the minimum number of pending requests.
	minCC := c.connClients[0]
	minReqs := minCC.PendingRequests()
	if minReqs == 0 {
		return minCC
	}
	for i := 1; i < len(c.connClients); i++ {
		cc := c.connClients[i]
		reqs := cc.PendingRequests()
		if reqs == 0 {
			return cc
		}
		if reqs < minReqs {
			minCC = cc
			minReqs = reqs
		}
	}

	maxConns := c.MaxConns
	if maxConns <= 0 {
		maxConns = 1
	}
	if len(c.connClients) < maxConns {
		return c.newConnClient()
	}
	return minCC
}

func (c *PipelineClient) newConnClient() *pipelineConnClient {
	cc := &pipelineConnClient{
		Addr:                          c.Addr,
		Name:                          c.Name,
		NoDefaultUserAgentHeader:      c.NoDefaultUserAgentHeader,
		MaxPendingRequests:            c.MaxPendingRequests,
		MaxBatchDelay:                 c.MaxBatchDelay,
		Dial:                          c.Dial,
		DialDualStack:                 c.DialDualStack,
		DisableHeaderNamesNormalizing: c.DisableHeaderNamesNormalizing,
		DisablePathNormalizing:        c.DisablePathNormalizing,
		IsTLS:                         c.IsTLS,
		TLSConfig:                     c.TLSConfig,
		MaxIdleConnDuration:           c.MaxIdleConnDuration,
		ReadBufferSize:                c.ReadBufferSize,
		WriteBufferSize:               c.WriteBufferSize,
		ReadTimeout:                   c.ReadTimeout,
		WriteTimeout:                  c.WriteTimeout,
		Logger:                        c.Logger,
	}
	c.connClients = append(c.connClients, cc)
	return cc
}

// ErrPipelineOverflow may be returned from PipelineClient.Do*
// if the requests' queue is overflown.
var ErrPipelineOverflow = errors.New("pipelined requests' queue has been overflown. Increase MaxConns and/or MaxPendingRequests")

// DefaultMaxPendingRequests is the default value
// for PipelineClient.MaxPendingRequests.
const DefaultMaxPendingRequests = 1024

func (c *pipelineConnClient) init() {
	c.chLock.Lock()
	if c.chR == nil {
		maxPendingRequests := c.MaxPendingRequests
		if maxPendingRequests <= 0 {
			maxPendingRequests = DefaultMaxPendingRequests
		}
		c.chR = make(chan *pipelineWork, maxPendingRequests)
		if c.chW == nil {
			c.chW = make(chan *pipelineWork, maxPendingRequests)
		}
		go func() {
			// Keep restarting the worker if it fails (connection errors for example).
			for {
				if err := c.worker(); err != nil {
					c.logger().Printf("error in PipelineClient(%q): %v", c.Addr, err)
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						// Throttle client reconnections on timeout errors
						time.Sleep(time.Second)
					}
				} else {
					c.chLock.Lock()
					stop := len(c.chR) == 0 && len(c.chW) == 0
					if !stop {
						c.chR = nil
						c.chW = nil
					}
					c.chLock.Unlock()

					if stop {
						break
					}
				}
			}
		}()
	}
	c.chLock.Unlock()
}

func (c *pipelineConnClient) worker() error {
	tlsConfig := c.cachedTLSConfig()
	conn, err := dialAddr(c.Addr, c.Dial, c.DialDualStack, c.IsTLS, tlsConfig, c.WriteTimeout)
	if err != nil {
		return err
	}

	// Start reader and writer
	stopW := make(chan struct{})
	doneW := make(chan error)
	go func() {
		doneW <- c.writer(conn, stopW)
	}()
	stopR := make(chan struct{})
	doneR := make(chan error)
	go func() {
		doneR <- c.reader(conn, stopR)
	}()

	// Wait until reader and writer are stopped
	select {
	case err = <-doneW:
		conn.Close()
		close(stopR)
		<-doneR
	case err = <-doneR:
		conn.Close()
		close(stopW)
		<-doneW
	}

	// Notify pending readers
	for len(c.chR) > 0 {
		w := <-c.chR
		w.err = errPipelineConnStopped
		w.done <- struct{}{}
	}

	return err
}

func (c *pipelineConnClient) cachedTLSConfig() *tls.Config {
	if !c.IsTLS {
		return nil
	}

	c.tlsConfigLock.Lock()
	cfg := c.tlsConfig
	if cfg == nil {
		cfg = newClientTLSConfig(c.TLSConfig, c.Addr)
		c.tlsConfig = cfg
	}
	c.tlsConfigLock.Unlock()

	return cfg
}

func (c *pipelineConnClient) writer(conn net.Conn, stopCh <-chan struct{}) error {
	writeBufferSize := c.WriteBufferSize
	if writeBufferSize <= 0 {
		writeBufferSize = defaultWriteBufferSize
	}
	bw := bufio.NewWriterSize(conn, writeBufferSize)
	defer bw.Flush()
	chR := c.chR
	chW := c.chW
	writeTimeout := c.WriteTimeout

	maxIdleConnDuration := c.MaxIdleConnDuration
	if maxIdleConnDuration <= 0 {
		maxIdleConnDuration = DefaultMaxIdleConnDuration
	}
	maxBatchDelay := c.MaxBatchDelay

	var (
		stopTimer      = time.NewTimer(time.Hour)
		flushTimer     = time.NewTimer(time.Hour)
		flushTimerCh   <-chan time.Time
		instantTimerCh = make(chan time.Time)

		w   *pipelineWork
		err error
	)
	close(instantTimerCh)
	for {
	againChW:
		select {
		case w = <-chW:
			// Fast path: len(chW) > 0
		default:
			// Slow path
			stopTimer.Reset(maxIdleConnDuration)
			select {
			case w = <-chW:
			case <-stopTimer.C:
				return nil
			case <-stopCh:
				return nil
			case <-flushTimerCh:
				if err = bw.Flush(); err != nil {
					return err
				}
				flushTimerCh = nil
				goto againChW
			}
		}

		if !w.deadline.IsZero() && time.Since(w.deadline) >= 0 {
			w.err = ErrTimeout
			w.done <- struct{}{}
			continue
		}

		w.resp.parseNetConn(conn)

		if writeTimeout > 0 {
			// Set Deadline every time, since golang has fixed the performance issue
			// See https://github.com/golang/go/issues/15133#issuecomment-271571395 for details
			currentTime := time.Now()
			if err = conn.SetWriteDeadline(currentTime.Add(writeTimeout)); err != nil {
				w.err = err
				w.done <- struct{}{}
				return err
			}
		}
		if err = w.req.Write(bw); err != nil {
			w.err = err
			w.done <- struct{}{}
			return err
		}
		if flushTimerCh == nil && (len(chW) == 0 || len(chR) == cap(chR)) {
			if maxBatchDelay > 0 {
				flushTimer.Reset(maxBatchDelay)
				flushTimerCh = flushTimer.C
			} else {
				flushTimerCh = instantTimerCh
			}
		}

	againChR:
		select {
		case chR <- w:
			// Fast path: len(chR) < cap(chR)
		default:
			// Slow path
			select {
			case chR <- w:
			case <-stopCh:
				w.err = errPipelineConnStopped
				w.done <- struct{}{}
				return nil
			case <-flushTimerCh:
				if err = bw.Flush(); err != nil {
					w.err = err
					w.done <- struct{}{}
					return err
				}
				flushTimerCh = nil
				goto againChR
			}
		}
	}
}

func (c *pipelineConnClient) reader(conn net.Conn, stopCh <-chan struct{}) error {
	readBufferSize := c.ReadBufferSize
	if readBufferSize <= 0 {
		readBufferSize = defaultReadBufferSize
	}
	br := bufio.NewReaderSize(conn, readBufferSize)
	chR := c.chR
	readTimeout := c.ReadTimeout

	var (
		w   *pipelineWork
		err error
	)
	for {
		select {
		case w = <-chR:
			// Fast path: len(chR) > 0
		default:
			// Slow path
			select {
			case w = <-chR:
			case <-stopCh:
				return nil
			}
		}

		if readTimeout > 0 {
			// Set Deadline every time, since golang has fixed the performance issue
			// See https://github.com/golang/go/issues/15133#issuecomment-271571395 for details
			currentTime := time.Now()
			if err = conn.SetReadDeadline(currentTime.Add(readTimeout)); err != nil {
				w.err = err
				w.done <- struct{}{}
				return err
			}
		}
		if err = w.resp.Read(br); err != nil {
			w.err = err
			w.done <- struct{}{}
			return err
		}

		w.done <- struct{}{}
	}
}

func (c *pipelineConnClient) logger() Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return defaultLogger
}

// PendingRequests returns the current number of pending requests pipelined
// to the server.
//
// This number may exceed MaxPendingRequests*MaxConns by up to two times, since
// each connection to the server may keep up to MaxPendingRequests requests
// in the queue before sending them to the server.
//
// This function may be used for balancing load among multiple PipelineClient
// instances.
func (c *PipelineClient) PendingRequests() int {
	c.connClientsLock.Lock()
	n := 0
	for _, cc := range c.connClients {
		n += cc.PendingRequests()
	}
	c.connClientsLock.Unlock()
	return n
}

func (c *pipelineConnClient) PendingRequests() int {
	c.init()

	c.chLock.Lock()
	n := len(c.chR) + len(c.chW)
	c.chLock.Unlock()
	return n
}

var errPipelineConnStopped = errors.New("pipeline connection has been stopped")
