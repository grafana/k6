package clientcert

import (
	"context"
	"errors"
	"html"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

const (
	// responseTimeout bounds how long a 503 response may keep the browser connection open.
	responseTimeout = 2 * time.Second

	relayBufferSize = 32 << 10
)

type upstreamResult struct {
	err          error
	gotFirstByte bool
}

type browserResult struct {
	err                 error
	browserEOF          bool
	upstreamWriteFailed bool
}

// relayState is shared by relay's coordinator and its two copying goroutines. The browserRead
// and replay fields are written only by the browser goroutine and read by the coordinator
// after that goroutine has reported its result.
type relayState struct {
	ctx                   context.Context //nolint:containedctx // relay's context, checked before a 503
	browser, browserRaw   net.Conn
	upstream, upstreamRaw net.Conn
	protocol              string
	replayLimit           int
	firstByte             atomic.Bool
	browserRead           bool
	replay                []byte
	replayOverflow        bool
	upstreamDone          chan upstreamResult
	browserDone           chan browserResult
}

// relay copies data between the browser and upstream TLS connections until both directions
// end. browser and upstream carry the data; browserRaw and upstreamRaw are their underlying
// connections, used for closing. When the server rejects the client certificate after the
// TLS 1.3 handshake, before sending any data, the browser receives a 503 response. For h2, the
// browser bytes read before that, up to replayLimit, are replayed to the 503 server.
func relay(ctx context.Context, browser, browserRaw, upstream, upstreamRaw net.Conn, protocol string, replayLimit int) {
	r := &relayState{
		ctx:          ctx,
		browser:      browser,
		browserRaw:   browserRaw,
		upstream:     upstream,
		upstreamRaw:  upstreamRaw,
		protocol:     protocol,
		replayLimit:  replayLimit,
		upstreamDone: make(chan upstreamResult, 1),
		browserDone:  make(chan browserResult, 1),
	}
	stop := context.AfterFunc(ctx, r.closeBoth)
	defer stop()

	go func() { r.upstreamDone <- r.copyUpstream() }()
	go func() { r.browserDone <- r.copyBrowser() }()

	select {
	case u := <-r.upstreamDone:
		r.onUpstreamDone(u)
	case b := <-r.browserDone:
		r.onBrowserDone(b)
	}
}

// copyUpstream copies upstream to browser until either fails.
func (r *relayState) copyUpstream() upstreamResult {
	buf := make([]byte, relayBufferSize)
	gotFirstByte := false
	for {
		n, err := r.upstream.Read(buf)
		if n > 0 {
			if !gotFirstByte {
				gotFirstByte = true
				r.firstByte.Store(true)
			}
			if _, werr := r.browser.Write(buf[:n]); werr != nil {
				return upstreamResult{err: werr, gotFirstByte: true}
			}
		}
		if err != nil {
			return upstreamResult{err: err, gotFirstByte: gotFirstByte}
		}
	}
}

// copyBrowser copies browser to upstream until either fails, half-closing upstream on the
// browser's EOF. For h2 it records what it reads until the first upstream byte.
func (r *relayState) copyBrowser() browserResult {
	buf := make([]byte, relayBufferSize)
	for {
		n, err := r.browser.Read(buf)
		if n > 0 {
			r.browserRead = true
			r.record(buf[:n])
			if _, werr := r.upstream.Write(buf[:n]); werr != nil {
				return browserResult{err: werr, upstreamWriteFailed: true}
			}
		}
		if errors.Is(err, io.EOF) {
			_ = closeWrite(r.upstream)
			return browserResult{err: err, browserEOF: true}
		}
		if err != nil {
			return browserResult{err: err}
		}
	}
}

// record appends b to the h2 replay buffer until the first upstream byte, marking an overflow
// beyond replayLimit.
func (r *relayState) record(b []byte) {
	if r.protocol != "h2" || r.replayOverflow || r.firstByte.Load() {
		return
	}
	if len(r.replay)+len(b) > r.replayLimit {
		r.replayOverflow = true
		r.replay = nil
		return
	}
	r.replay = append(r.replay, b...)
}

func (r *relayState) onUpstreamDone(u upstreamResult) {
	switch {
	case errors.Is(u.err, io.EOF):
		_ = closeWrite(r.browser)
		<-r.browserDone
		r.closeBoth()
	case u.gotFirstByte:
		r.closeBoth()
		<-r.browserDone
	default:
		// The server failed before sending data, e.g. by rejecting the client certificate
		// after the TLS 1.3 handshake. Unblock the browser goroutine's write or read.
		_ = r.upstreamRaw.Close()
		_ = r.browser.SetReadDeadline(time.Now())
		r.reject(<-r.browserDone, u.err)
	}
}

func (r *relayState) onBrowserDone(b browserResult) {
	switch {
	case b.browserEOF:
		<-r.upstreamDone
		r.closeBoth()
	case b.upstreamWriteFailed:
		_ = r.upstreamRaw.Close()
		u := <-r.upstreamDone
		if u.gotFirstByte {
			r.closeBoth()
			return
		}
		err := b.err
		if !errors.Is(u.err, net.ErrClosed) {
			err = u.err
		}
		r.reject(b, err)
	default:
		_ = r.upstreamRaw.Close()
		<-r.upstreamDone
		_ = r.browserRaw.Close()
	}
}

// reject reports err to the browser with a 503 response, unless the browser has already
// closed its side, then closes both connections.
func (r *relayState) reject(b browserResult, err error) {
	defer r.closeBoth()
	if b.browserEOF || r.ctx.Err() != nil {
		return
	}
	if derr := r.browser.SetReadDeadline(time.Time{}); derr != nil {
		return
	}
	switch {
	case r.protocol != "h2":
		write503(r.browser, r.browserRaw, !r.browserRead, err)
	case !r.replayOverflow:
		serve503H2(r.browser, r.browserRaw, r.replay, err)
	}
}

func (r *relayState) closeBoth() {
	_ = r.browserRaw.Close()
	_ = r.upstreamRaw.Close()
}

func closeWrite(conn net.Conn) error {
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return errors.ErrUnsupported
}

// clientCertificateErrorBody returns the HTML body of a 503 response reporting err.
func clientCertificateErrorBody(err error) string {
	return strings.ReplaceAll(html.EscapeString("k6 client-certificate error: "+err.Error()), "\n", " <br>")
}

// write503 writes an HTTP/1.1 503 response reporting err. With awaitRequest, it first waits
// up to responseTimeout for the browser to start its request, so that the response is not
// unsolicited. It then half-closes browser and discards what the browser sends until it
// closes or responseTimeout passes, so that unread request bytes do not make the closing
// connection reset and discard the response.
func write503(browser, browserRaw net.Conn, awaitRequest bool, err error) {
	if awaitRequest && !awaitBrowserRequest(browser) {
		return
	}
	body := clientCertificateErrorBody(err)
	resp := "HTTP/1.1 503 Internal Server Error\r\n" +
		"Content-Type: text/html; charset=utf-8\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n" +
		"\r\n" +
		body
	if _, werr := io.WriteString(browser, resp); werr != nil {
		return
	}
	_ = closeWrite(browser)
	if derr := browserRaw.SetReadDeadline(time.Now().Add(responseTimeout)); derr != nil {
		return
	}
	_, _ = io.Copy(io.Discard, browserRaw)
}

// awaitBrowserRequest waits up to responseTimeout for the browser's first bytes and discards
// them. It reports false if the browser closed or failed without sending any.
func awaitBrowserRequest(browser net.Conn) bool {
	if err := browser.SetReadDeadline(time.Now().Add(responseTimeout)); err != nil {
		return false
	}
	var buf [512]byte
	n, err := browser.Read(buf[:])
	if derr := browser.SetReadDeadline(time.Time{}); derr != nil {
		return false
	}
	var netErr net.Error
	return n > 0 || err == nil || (errors.As(err, &netErr) && netErr.Timeout())
}

// serve503H2 answers every HTTP/2 request on browser with a 503 response reporting err,
// reading replay before browser. It returns once the HTTP/2 server has stopped, closing
// browserRaw if that takes longer than responseTimeout.
func serve503H2(browser, browserRaw net.Conn, replay []byte, err error) {
	body := clientCertificateErrorBody(err)
	responded := make(chan struct{})
	var respondedOnce sync.Once
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The HTTP/2 server turns Connection: close into GOAWAY and closes the connection
		// when it is idle.
		w.Header().Set("Connection", "close")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		respondedOnce.Do(func() { close(responded) })
	})

	conn := browser
	if len(replay) > 0 {
		conn = &replayConn{Conn: browser, replay: replay}
	}
	served := make(chan struct{})
	go func() {
		defer close(served)
		//nolint:staticcheck // serves an already accepted connection, which http.Server cannot
		(&http2.Server{}).ServeConn(conn, &http2.ServeConnOpts{
			Handler:    handler,
			BaseConfig: &http.Server{ErrorLog: stdlog.New(io.Discard, "", 0)}, //nolint:gosec // serves one response
		})
	}()

	timer := time.NewTimer(responseTimeout)
	defer timer.Stop()
	for {
		select {
		case <-responded:
			responded = nil
		case <-served:
			return
		case <-timer.C:
			_ = browserRaw.Close()
			<-served
			return
		}
	}
}

// replayConn reads replay before reading the embedded connection.
type replayConn struct {
	net.Conn
	replay []byte
}

func (c *replayConn) Read(b []byte) (int, error) {
	if len(c.replay) > 0 {
		n := copy(b, c.replay)
		c.replay = c.replay[n:]
		return n, nil
	}
	return c.Conn.Read(b)
}
