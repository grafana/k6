package fasthttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"sync"
)

// AcquireURI returns an empty URI instance from the pool.
//
// Release the URI with ReleaseURI after the URI is no longer needed.
// This allows reducing GC load.
func AcquireURI() *URI {
	return uriPool.Get().(*URI)
}

// ReleaseURI releases the URI acquired via AcquireURI.
//
// The released URI mustn't be used after releasing it, otherwise data races
// may occur.
func ReleaseURI(u *URI) {
	u.Reset()
	uriPool.Put(u)
}

var uriPool = &sync.Pool{
	New: func() interface{} {
		return &URI{}
	},
}

// URI represents URI :) .
//
// It is forbidden copying URI instances. Create new instance and use CopyTo
// instead.
//
// URI instance MUST NOT be used from concurrently running goroutines.
type URI struct {
	noCopy noCopy

	pathOriginal []byte
	scheme       []byte
	path         []byte
	queryString  []byte
	hash         []byte
	host         []byte

	queryArgs       Args
	parsedQueryArgs bool

	// Path values are sent as-is without normalization
	//
	// Disabled path normalization may be useful for proxying incoming requests
	// to servers that are expecting paths to be forwarded as-is.
	//
	// By default path values are normalized, i.e.
	// extra slashes are removed, special characters are encoded.
	DisablePathNormalizing bool

	fullURI    []byte
	requestURI []byte

	username []byte
	password []byte
}

// CopyTo copies uri contents to dst.
func (u *URI) CopyTo(dst *URI) {
	dst.Reset()
	dst.pathOriginal = append(dst.pathOriginal, u.pathOriginal...)
	dst.scheme = append(dst.scheme, u.scheme...)
	dst.path = append(dst.path, u.path...)
	dst.queryString = append(dst.queryString, u.queryString...)
	dst.hash = append(dst.hash, u.hash...)
	dst.host = append(dst.host, u.host...)
	dst.username = append(dst.username, u.username...)
	dst.password = append(dst.password, u.password...)

	u.queryArgs.CopyTo(&dst.queryArgs)
	dst.parsedQueryArgs = u.parsedQueryArgs
	dst.DisablePathNormalizing = u.DisablePathNormalizing

	// fullURI and requestURI shouldn't be copied, since they are created
	// from scratch on each FullURI() and RequestURI() call.
}

// Hash returns URI hash, i.e. qwe of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Hash() []byte {
	return u.hash
}

// SetHash sets URI hash.
func (u *URI) SetHash(hash string) {
	u.hash = append(u.hash[:0], hash...)
}

// SetHashBytes sets URI hash.
func (u *URI) SetHashBytes(hash []byte) {
	u.hash = append(u.hash[:0], hash...)
}

// Username returns URI username
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Username() []byte {
	return u.username
}

// SetUsername sets URI username.
func (u *URI) SetUsername(username string) {
	u.username = append(u.username[:0], username...)
}

// SetUsernameBytes sets URI username.
func (u *URI) SetUsernameBytes(username []byte) {
	u.username = append(u.username[:0], username...)
}

// Password returns URI password
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Password() []byte {
	return u.password
}

// SetPassword sets URI password.
func (u *URI) SetPassword(password string) {
	u.password = append(u.password[:0], password...)
}

// SetPasswordBytes sets URI password.
func (u *URI) SetPasswordBytes(password []byte) {
	u.password = append(u.password[:0], password...)
}

// QueryString returns URI query string,
// i.e. baz=123 of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned bytes are valid until the next URI method call.
func (u *URI) QueryString() []byte {
	return u.queryString
}

// SetQueryString sets URI query string.
func (u *URI) SetQueryString(queryString string) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// SetQueryStringBytes sets URI query string.
func (u *URI) SetQueryStringBytes(queryString []byte) {
	u.queryString = append(u.queryString[:0], queryString...)
	u.parsedQueryArgs = false
}

// Path returns URI path, i.e. /foo/bar of http://aaa.com/foo/bar?baz=123#qwe .
//
// The returned path is always urldecoded and normalized,
// i.e. '//f%20obar/baz/../zzz' becomes '/f obar/zzz'.
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Path() []byte {
	path := u.path
	if len(path) == 0 {
		path = strSlash
	}
	return path
}

// SetPath sets URI path.
func (u *URI) SetPath(path string) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// SetPathBytes sets URI path.
func (u *URI) SetPathBytes(path []byte) {
	u.pathOriginal = append(u.pathOriginal[:0], path...)
	u.path = normalizePath(u.path, u.pathOriginal)
}

// PathOriginal returns the original path from requestURI passed to URI.Parse().
//
// The returned bytes are valid until the next URI method call.
func (u *URI) PathOriginal() []byte {
	return u.pathOriginal
}

// Scheme returns URI scheme, i.e. http of http://aaa.com/foo/bar?baz=123#qwe .
//
// Returned scheme is always lowercased.
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Scheme() []byte {
	scheme := u.scheme
	if len(scheme) == 0 {
		scheme = strHTTP
	}
	return scheme
}

// SetScheme sets URI scheme, i.e. http, https, ftp, etc.
func (u *URI) SetScheme(scheme string) {
	u.scheme = append(u.scheme[:0], scheme...)
	lowercaseBytes(u.scheme)
}

// SetSchemeBytes sets URI scheme, i.e. http, https, ftp, etc.
func (u *URI) SetSchemeBytes(scheme []byte) {
	u.scheme = append(u.scheme[:0], scheme...)
	lowercaseBytes(u.scheme)
}

func (u *URI) isHTTPS() bool {
	return bytes.Equal(u.scheme, strHTTPS)
}

func (u *URI) isHTTP() bool {
	return len(u.scheme) == 0 || bytes.Equal(u.scheme, strHTTP)
}

// Reset clears uri.
func (u *URI) Reset() {
	u.pathOriginal = u.pathOriginal[:0]
	u.scheme = u.scheme[:0]
	u.path = u.path[:0]
	u.queryString = u.queryString[:0]
	u.hash = u.hash[:0]
	u.username = u.username[:0]
	u.password = u.password[:0]

	u.host = u.host[:0]
	u.queryArgs.Reset()
	u.parsedQueryArgs = false
	u.DisablePathNormalizing = false

	// There is no need in u.fullURI = u.fullURI[:0], since full uri
	// is calculated on each call to FullURI().

	// There is no need in u.requestURI = u.requestURI[:0], since requestURI
	// is calculated on each call to RequestURI().
}

// Host returns host part, i.e. aaa.com of http://aaa.com/foo/bar?baz=123#qwe .
//
// Host is always lowercased.
//
// The returned bytes are valid until the next URI method call.
func (u *URI) Host() []byte {
	return u.host
}

// SetHost sets host for the uri.
func (u *URI) SetHost(host string) {
	u.host = append(u.host[:0], host...)
	lowercaseBytes(u.host)
}

// SetHostBytes sets host for the uri.
func (u *URI) SetHostBytes(host []byte) {
	u.host = append(u.host[:0], host...)
	lowercaseBytes(u.host)
}

var (
	ErrorInvalidURI = errors.New("invalid uri")
)

// Parse initializes URI from the given host and uri.
//
// host may be nil. In this case uri must contain fully qualified uri,
// i.e. with scheme and host. http is assumed if scheme is omitted.
//
// uri may contain e.g. RequestURI without scheme and host if host is non-empty.
func (u *URI) Parse(host, uri []byte) error {
	return u.parse(host, uri, false)
}

func (u *URI) parse(host, uri []byte, isTLS bool) error {
	u.Reset()

	if stringContainsCTLByte(uri) {
		return ErrorInvalidURI
	}

	if len(host) == 0 || bytes.Contains(uri, strColonSlashSlash) {
		scheme, newHost, newURI := splitHostURI(host, uri)
		u.SetSchemeBytes(scheme)
		host = newHost
		uri = newURI
	}

	if isTLS {
		u.SetSchemeBytes(strHTTPS)
	}

	if n := bytes.IndexByte(host, '@'); n >= 0 {
		auth := host[:n]
		host = host[n+1:]

		if n := bytes.IndexByte(auth, ':'); n >= 0 {
			u.username = append(u.username[:0], auth[:n]...)
			u.password = append(u.password[:0], auth[n+1:]...)
		} else {
			u.username = append(u.username[:0], auth...)
			u.password = u.password[:0]
		}
	}

	u.host = append(u.host, host...)
	if parsedHost, err := parseHost(u.host); err != nil {
		return err
	} else {
		u.host = parsedHost
	}
	lowercaseBytes(u.host)

	b := uri
	queryIndex := bytes.IndexByte(b, '?')
	fragmentIndex := bytes.IndexByte(b, '#')
	// Ignore query in fragment part
	if fragmentIndex >= 0 && queryIndex > fragmentIndex {
		queryIndex = -1
	}

	if queryIndex < 0 && fragmentIndex < 0 {
		u.pathOriginal = append(u.pathOriginal, b...)
		u.path = normalizePath(u.path, u.pathOriginal)
		return nil
	}

	if queryIndex >= 0 {
		// Path is everything up to the start of the query
		u.pathOriginal = append(u.pathOriginal, b[:queryIndex]...)
		u.path = normalizePath(u.path, u.pathOriginal)

		if fragmentIndex < 0 {
			u.queryString = append(u.queryString, b[queryIndex+1:]...)
		} else {
			u.queryString = append(u.queryString, b[queryIndex+1:fragmentIndex]...)
			u.hash = append(u.hash, b[fragmentIndex+1:]...)
		}
		return nil
	}

	// fragmentIndex >= 0 && queryIndex < 0
	// Path is up to the start of fragment
	u.pathOriginal = append(u.pathOriginal, b[:fragmentIndex]...)
	u.path = normalizePath(u.path, u.pathOriginal)
	u.hash = append(u.hash, b[fragmentIndex+1:]...)

	return nil
}

// parseHost parses host as an authority without user
// information. That is, as host[:port].
//
// Based on https://github.com/golang/go/blob/8ac5cbe05d61df0a7a7c9a38ff33305d4dcfea32/src/net/url/url.go#L619
//
// The host is parsed and unescaped in place overwriting the contents of the host parameter.
func parseHost(host []byte) ([]byte, error) {
	if len(host) > 0 && host[0] == '[' {
		// Parse an IP-Literal in RFC 3986 and RFC 6874.
		// E.g., "[fe80::1]", "[fe80::1%25en0]", "[fe80::1]:80".
		i := bytes.LastIndexByte(host, ']')
		if i < 0 {
			return nil, errors.New("missing ']' in host")
		}
		colonPort := host[i+1:]
		if !validOptionalPort(colonPort) {
			return nil, fmt.Errorf("invalid port %q after host", colonPort)
		}

		// RFC 6874 defines that %25 (%-encoded percent) introduces
		// the zone identifier, and the zone identifier can use basically
		// any %-encoding it likes. That's different from the host, which
		// can only %-encode non-ASCII bytes.
		// We do impose some restrictions on the zone, to avoid stupidity
		// like newlines.
		zone := bytes.Index(host[:i], []byte("%25"))
		if zone >= 0 {
			host1, err := unescape(host[:zone], encodeHost)
			if err != nil {
				return nil, err
			}
			host2, err := unescape(host[zone:i], encodeZone)
			if err != nil {
				return nil, err
			}
			host3, err := unescape(host[i:], encodeHost)
			if err != nil {
				return nil, err
			}
			return append(host1, append(host2, host3...)...), nil
		}
	} else if i := bytes.LastIndexByte(host, ':'); i != -1 {
		colonPort := host[i:]
		if !validOptionalPort(colonPort) {
			return nil, fmt.Errorf("invalid port %q after host", colonPort)
		}
	}

	var err error
	if host, err = unescape(host, encodeHost); err != nil {
		return nil, err
	}
	return host, nil
}

type encoding int

const (
	encodeHost encoding = 1 + iota
	encodeZone
)

type EscapeError string

func (e EscapeError) Error() string {
	return "invalid URL escape " + strconv.Quote(string(e))
}

type InvalidHostError string

func (e InvalidHostError) Error() string {
	return "invalid character " + strconv.Quote(string(e)) + " in host name"
}

// unescape unescapes a string; the mode specifies
// which section of the URL string is being unescaped.
//
// Based on https://github.com/golang/go/blob/8ac5cbe05d61df0a7a7c9a38ff33305d4dcfea32/src/net/url/url.go#L199
//
// Unescapes in place overwriting the contents of s and returning it.
func unescape(s []byte, mode encoding) ([]byte, error) {
	// Count %, check that they're well-formed.
	n := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			n++
			if i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2]) {
				s = s[i:]
				if len(s) > 3 {
					s = s[:3]
				}
				return nil, EscapeError(s)
			}
			// Per https://tools.ietf.org/html/rfc3986#page-21
			// in the host component %-encoding can only be used
			// for non-ASCII bytes.
			// But https://tools.ietf.org/html/rfc6874#section-2
			// introduces %25 being allowed to escape a percent sign
			// in IPv6 scoped-address literals. Yay.
			if mode == encodeHost && unhex(s[i+1]) < 8 && !bytes.Equal(s[i:i+3], []byte("%25")) {
				return nil, EscapeError(s[i : i+3])
			}
			if mode == encodeZone {
				// RFC 6874 says basically "anything goes" for zone identifiers
				// and that even non-ASCII can be redundantly escaped,
				// but it seems prudent to restrict %-escaped bytes here to those
				// that are valid host name bytes in their unescaped form.
				// That is, you can use escaping in the zone identifier but not
				// to introduce bytes you couldn't just write directly.
				// But Windows puts spaces here! Yay.
				v := unhex(s[i+1])<<4 | unhex(s[i+2])
				if !bytes.Equal(s[i:i+3], []byte("%25")) && v != ' ' && shouldEscape(v, encodeHost) {
					return nil, EscapeError(s[i : i+3])
				}
			}
			i += 3
		default:
			if (mode == encodeHost || mode == encodeZone) && s[i] < 0x80 && shouldEscape(s[i], mode) {
				return nil, InvalidHostError(s[i : i+1])
			}
			i++
		}
	}

	if n == 0 {
		return s, nil
	}

	t := s[:0]
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%':
			t = append(t, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 2
		default:
			t = append(t, s[i])
		}
	}
	return t, nil
}

// Return true if the specified character should be escaped when
// appearing in a URL string, according to RFC 3986.
//
// Please be informed that for now shouldEscape does not check all
// reserved characters correctly. See https://github.com/golang/go/issues/5684.
//
// Based on https://github.com/golang/go/blob/8ac5cbe05d61df0a7a7c9a38ff33305d4dcfea32/src/net/url/url.go#L100
func shouldEscape(c byte, mode encoding) bool {
	// §2.3 Unreserved characters (alphanum)
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' {
		return false
	}

	if mode == encodeHost || mode == encodeZone {
		// §3.2.2 Host allows
		//	sub-delims = "!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" / "," / ";" / "="
		// as part of reg-name.
		// We add : because we include :port as part of host.
		// We add [ ] because we include [ipv6]:port as part of host.
		// We add < > because they're the only characters left that
		// we could possibly allow, and Parse will reject them if we
		// escape them (because hosts can't use %-encoding for
		// ASCII bytes).
		switch c {
		case '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', ':', '[', ']', '<', '>', '"':
			return false
		}
	}

	if c == '-' || c == '_' || c == '.' || c == '~' { // §2.3 Unreserved characters (mark)
		return false
	}

	// Everything else must be escaped.
	return true
}

func ishex(c byte) bool {
	return ('0' <= c && c <= '9') ||
		('a' <= c && c <= 'f') ||
		('A' <= c && c <= 'F')
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// validOptionalPort reports whether port is either an empty string
// or matches /^:\d*$/
func validOptionalPort(port []byte) bool {
	if len(port) == 0 {
		return true
	}
	if port[0] != ':' {
		return false
	}
	for _, b := range port[1:] {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func normalizePath(dst, src []byte) []byte {
	dst = dst[:0]
	dst = addLeadingSlash(dst, src)
	dst = decodeArgAppendNoPlus(dst, src)

	// remove duplicate slashes
	b := dst
	bSize := len(b)
	for {
		n := bytes.Index(b, strSlashSlash)
		if n < 0 {
			break
		}
		b = b[n:]
		copy(b, b[1:])
		b = b[:len(b)-1]
		bSize--
	}
	dst = dst[:bSize]

	// remove /./ parts
	b = dst
	for {
		n := bytes.Index(b, strSlashDotSlash)
		if n < 0 {
			break
		}
		nn := n + len(strSlashDotSlash) - 1
		copy(b[n:], b[nn:])
		b = b[:len(b)-nn+n]
	}

	// remove /foo/../ parts
	for {
		n := bytes.Index(b, strSlashDotDotSlash)
		if n < 0 {
			break
		}
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			nn = 0
		}
		n += len(strSlashDotDotSlash) - 1
		copy(b[nn:], b[n:])
		b = b[:len(b)-n+nn]
	}

	// remove trailing /foo/..
	n := bytes.LastIndex(b, strSlashDotDot)
	if n >= 0 && n+len(strSlashDotDot) == len(b) {
		nn := bytes.LastIndexByte(b[:n], '/')
		if nn < 0 {
			return append(dst[:0], strSlash...)
		}
		b = b[:nn+1]
	}

	if filepath.Separator == '\\' {
		// remove \.\ parts
		for {
			n := bytes.Index(b, strBackSlashDotBackSlash)
			if n < 0 {
				break
			}
			nn := n + len(strSlashDotSlash) - 1
			copy(b[n:], b[nn:])
			b = b[:len(b)-nn+n]
		}

		// remove /foo/..\ parts
		for {
			n := bytes.Index(b, strSlashDotDotBackSlash)
			if n < 0 {
				break
			}
			nn := bytes.LastIndexByte(b[:n], '/')
			if nn < 0 {
				nn = 0
			}
			nn++
			n += len(strSlashDotDotBackSlash)
			copy(b[nn:], b[n:])
			b = b[:len(b)-n+nn]
		}

		// remove /foo\..\ parts
		for {
			n := bytes.Index(b, strBackSlashDotDotBackSlash)
			if n < 0 {
				break
			}
			nn := bytes.LastIndexByte(b[:n], '/')
			if nn < 0 {
				nn = 0
			}
			n += len(strBackSlashDotDotBackSlash) - 1
			copy(b[nn:], b[n:])
			b = b[:len(b)-n+nn]
		}

		// remove trailing \foo\..
		n := bytes.LastIndex(b, strBackSlashDotDot)
		if n >= 0 && n+len(strSlashDotDot) == len(b) {
			nn := bytes.LastIndexByte(b[:n], '/')
			if nn < 0 {
				return append(dst[:0], strSlash...)
			}
			b = b[:nn+1]
		}
	}

	return b
}

// RequestURI returns RequestURI - i.e. URI without Scheme and Host.
func (u *URI) RequestURI() []byte {
	var dst []byte
	if u.DisablePathNormalizing {
		dst = append(u.requestURI[:0], u.PathOriginal()...)
	} else {
		dst = appendQuotedPath(u.requestURI[:0], u.Path())
	}
	if u.parsedQueryArgs && u.queryArgs.Len() > 0 {
		dst = append(dst, '?')
		dst = u.queryArgs.AppendBytes(dst)
	} else if len(u.queryString) > 0 {
		dst = append(dst, '?')
		dst = append(dst, u.queryString...)
	}
	u.requestURI = dst
	return u.requestURI
}

// LastPathSegment returns the last part of uri path after '/'.
//
// Examples:
//
//   - For /foo/bar/baz.html path returns baz.html.
//   - For /foo/bar/ returns empty byte slice.
//   - For /foobar.js returns foobar.js.
//
// The returned bytes are valid until the next URI method call.
func (u *URI) LastPathSegment() []byte {
	path := u.Path()
	n := bytes.LastIndexByte(path, '/')
	if n < 0 {
		return path
	}
	return path[n+1:]
}

// Update updates uri.
//
// The following newURI types are accepted:
//
//   - Absolute, i.e. http://foobar.com/aaa/bb?cc . In this case the original
//     uri is replaced by newURI.
//   - Absolute without scheme, i.e. //foobar.com/aaa/bb?cc. In this case
//     the original scheme is preserved.
//   - Missing host, i.e. /aaa/bb?cc . In this case only RequestURI part
//     of the original uri is replaced.
//   - Relative path, i.e.  xx?yy=abc . In this case the original RequestURI
//     is updated according to the new relative path.
func (u *URI) Update(newURI string) {
	u.UpdateBytes(s2b(newURI))
}

// UpdateBytes updates uri.
//
// The following newURI types are accepted:
//
//   - Absolute, i.e. http://foobar.com/aaa/bb?cc . In this case the original
//     uri is replaced by newURI.
//   - Absolute without scheme, i.e. //foobar.com/aaa/bb?cc. In this case
//     the original scheme is preserved.
//   - Missing host, i.e. /aaa/bb?cc . In this case only RequestURI part
//     of the original uri is replaced.
//   - Relative path, i.e.  xx?yy=abc . In this case the original RequestURI
//     is updated according to the new relative path.
func (u *URI) UpdateBytes(newURI []byte) {
	u.requestURI = u.updateBytes(newURI, u.requestURI)
}

func (u *URI) updateBytes(newURI, buf []byte) []byte {
	if len(newURI) == 0 {
		return buf
	}

	n := bytes.Index(newURI, strSlashSlash)
	if n >= 0 {
		// absolute uri
		var b [32]byte
		schemeOriginal := b[:0]
		if len(u.scheme) > 0 {
			schemeOriginal = append([]byte(nil), u.scheme...)
		}
		if err := u.Parse(nil, newURI); err != nil {
			return nil
		}
		if len(schemeOriginal) > 0 && len(u.scheme) == 0 {
			u.scheme = append(u.scheme[:0], schemeOriginal...)
		}
		return buf
	}

	if newURI[0] == '/' {
		// uri without host
		buf = u.appendSchemeHost(buf[:0])
		buf = append(buf, newURI...)
		if err := u.Parse(nil, buf); err != nil {
			return nil
		}
		return buf
	}

	// relative path
	switch newURI[0] {
	case '?':
		// query string only update
		u.SetQueryStringBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	case '#':
		// update only hash
		u.SetHashBytes(newURI[1:])
		return append(buf[:0], u.FullURI()...)
	default:
		// update the last path part after the slash
		path := u.Path()
		n = bytes.LastIndexByte(path, '/')
		if n < 0 {
			panic(fmt.Sprintf("BUG: path must contain at least one slash: %q %q", u.Path(), newURI))
		}
		buf = u.appendSchemeHost(buf[:0])
		buf = appendQuotedPath(buf, path[:n+1])
		buf = append(buf, newURI...)
		if err := u.Parse(nil, buf); err != nil {
			return nil
		}
		return buf
	}
}

// FullURI returns full uri in the form {Scheme}://{Host}{RequestURI}#{Hash}.
//
// The returned bytes are valid until the next URI method call.
func (u *URI) FullURI() []byte {
	u.fullURI = u.AppendBytes(u.fullURI[:0])
	return u.fullURI
}

// AppendBytes appends full uri to dst and returns the extended dst.
func (u *URI) AppendBytes(dst []byte) []byte {
	dst = u.appendSchemeHost(dst)
	dst = append(dst, u.RequestURI()...)
	if len(u.hash) > 0 {
		dst = append(dst, '#')
		dst = append(dst, u.hash...)
	}
	return dst
}

func (u *URI) appendSchemeHost(dst []byte) []byte {
	dst = append(dst, u.Scheme()...)
	dst = append(dst, strColonSlashSlash...)
	return append(dst, u.Host()...)
}

// WriteTo writes full uri to w.
//
// WriteTo implements io.WriterTo interface.
func (u *URI) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(u.FullURI())
	return int64(n), err
}

// String returns full uri.
func (u *URI) String() string {
	return string(u.FullURI())
}

func splitHostURI(host, uri []byte) ([]byte, []byte, []byte) {
	n := bytes.Index(uri, strSlashSlash)
	if n < 0 {
		return strHTTP, host, uri
	}
	scheme := uri[:n]
	if bytes.IndexByte(scheme, '/') >= 0 {
		return strHTTP, host, uri
	}
	if len(scheme) > 0 && scheme[len(scheme)-1] == ':' {
		scheme = scheme[:len(scheme)-1]
	}
	n += len(strSlashSlash)
	uri = uri[n:]
	n = bytes.IndexByte(uri, '/')
	nq := bytes.IndexByte(uri, '?')
	if nq >= 0 && nq < n {
		// A hack for urls like foobar.com?a=b/xyz
		n = nq
	} else if n < 0 {
		// A hack for bogus urls like foobar.com?a=b without
		// slash after host.
		if nq >= 0 {
			return scheme, uri[:nq], uri[nq:]
		}
		return scheme, uri, strSlash
	}
	return scheme, uri[:n], uri[n:]
}

// QueryArgs returns query args.
//
// The returned args are valid until the next URI method call.
func (u *URI) QueryArgs() *Args {
	u.parseQueryArgs()
	return &u.queryArgs
}

func (u *URI) parseQueryArgs() {
	if u.parsedQueryArgs {
		return
	}
	u.queryArgs.ParseBytes(u.queryString)
	u.parsedQueryArgs = true
}

// stringContainsCTLByte reports whether s contains any ASCII control character.
func stringContainsCTLByte(s []byte) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < ' ' || b == 0x7f {
			return true
		}
	}
	return false
}
