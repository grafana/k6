package http3

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/http/httpguts"

	"github.com/quic-go/qpack"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3/qlog"
	"github.com/quic-go/quic-go/qlogwriter"
)

type qpackError struct{ err error }

func (e *qpackError) Error() string { return fmt.Sprintf("qpack: %v", e.err) }
func (e *qpackError) Unwrap() error { return e.err }

var errHeaderTooLarge = errors.New("http3: headers too large")

type header struct {
	// Pseudo header fields defined in RFC 9114
	Path      string
	Method    string
	Authority string
	Scheme    string
	Status    string
	// for Extended connect
	Protocol string
	// parsed and deduplicated. -1 if no Content-Length header is sent
	ContentLength int64
	// all non-pseudo headers
	Headers http.Header
}

// connection-specific header fields must not be sent on HTTP/3
var invalidHeaderFields = [...]string{
	"connection",
	"keep-alive",
	"proxy-connection",
	"transfer-encoding",
	"upgrade",
}

func parseHeaders(decodeFn qpack.DecodeFunc, isRequest bool, sizeLimit int, headerFields *[]qpack.HeaderField) (header, error) {
	hdr := header{Headers: make(http.Header)}
	var readFirstRegularHeader, readContentLength bool
	var contentLengthStr string
	for {
		h, err := decodeFn()
		if err != nil {
			if err == io.EOF {
				break
			}
			return header{}, &qpackError{err}
		}
		if headerFields != nil {
			*headerFields = append(*headerFields, h)
		}
		// RFC 9114, section 4.2.2:
		// The size of a field list is calculated based on the uncompressed size of fields,
		// including the length of the name and value in bytes plus an overhead of 32 bytes for each field.
		sizeLimit -= len(h.Name) + len(h.Value) + 32
		if sizeLimit < 0 {
			return header{}, errHeaderTooLarge
		}
		// field names need to be lowercase, see section 4.2 of RFC 9114
		if strings.ToLower(h.Name) != h.Name {
			return header{}, fmt.Errorf("header field is not lower-case: %s", h.Name)
		}
		if !httpguts.ValidHeaderFieldValue(h.Value) {
			return header{}, fmt.Errorf("invalid header field value for %s: %q", h.Name, h.Value)
		}
		if h.IsPseudo() {
			if readFirstRegularHeader {
				// all pseudo headers must appear before regular header fields, see section 4.3 of RFC 9114
				return header{}, fmt.Errorf("received pseudo header %s after a regular header field", h.Name)
			}
			var isResponsePseudoHeader bool  // pseudo headers are either valid for requests or for responses
			var isDuplicatePseudoHeader bool // pseudo headers are allowed to appear exactly once
			switch h.Name {
			case ":path":
				isDuplicatePseudoHeader = hdr.Path != ""
				hdr.Path = h.Value
			case ":method":
				isDuplicatePseudoHeader = hdr.Method != ""
				hdr.Method = h.Value
			case ":authority":
				isDuplicatePseudoHeader = hdr.Authority != ""
				hdr.Authority = h.Value
			case ":protocol":
				isDuplicatePseudoHeader = hdr.Protocol != ""
				hdr.Protocol = h.Value
			case ":scheme":
				isDuplicatePseudoHeader = hdr.Scheme != ""
				hdr.Scheme = h.Value
			case ":status":
				isDuplicatePseudoHeader = hdr.Status != ""
				hdr.Status = h.Value
				isResponsePseudoHeader = true
			default:
				return header{}, fmt.Errorf("unknown pseudo header: %s", h.Name)
			}
			if isDuplicatePseudoHeader {
				return header{}, fmt.Errorf("duplicate pseudo header: %s", h.Name)
			}
			if isRequest && isResponsePseudoHeader {
				return header{}, fmt.Errorf("invalid request pseudo header: %s", h.Name)
			}
			if !isRequest && !isResponsePseudoHeader {
				return header{}, fmt.Errorf("invalid response pseudo header: %s", h.Name)
			}
		} else {
			if !httpguts.ValidHeaderFieldName(h.Name) {
				return header{}, fmt.Errorf("invalid header field name: %q", h.Name)
			}
			for _, invalidField := range invalidHeaderFields {
				if h.Name == invalidField {
					return header{}, fmt.Errorf("invalid header field name: %q", h.Name)
				}
			}
			if h.Name == "te" && h.Value != "trailers" {
				return header{}, fmt.Errorf("invalid TE header field value: %q", h.Value)
			}
			readFirstRegularHeader = true
			switch h.Name {
			case "content-length":
				// Ignore duplicate Content-Length headers.
				// Fail if the duplicates differ.
				if !readContentLength {
					readContentLength = true
					contentLengthStr = h.Value
				} else if contentLengthStr != h.Value {
					return header{}, fmt.Errorf("contradicting content lengths (%s and %s)", contentLengthStr, h.Value)
				}
			default:
				hdr.Headers.Add(h.Name, h.Value)
			}
		}
	}
	hdr.ContentLength = -1
	if len(contentLengthStr) > 0 {
		// use ParseUint instead of ParseInt, so that parsing fails on negative values
		cl, err := strconv.ParseUint(contentLengthStr, 10, 63)
		if err != nil {
			return header{}, fmt.Errorf("invalid content length: %w", err)
		}
		hdr.Headers.Set("Content-Length", contentLengthStr)
		hdr.ContentLength = int64(cl)
	}
	return hdr, nil
}

func parseTrailers(decodeFn qpack.DecodeFunc, headerFields *[]qpack.HeaderField) (http.Header, error) {
	h := make(http.Header)
	for {
		hf, err := decodeFn()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, &qpackError{err}
		}
		if headerFields != nil {
			*headerFields = append(*headerFields, hf)
		}
		if hf.IsPseudo() {
			return nil, fmt.Errorf("http3: received pseudo header in trailer: %s", hf.Name)
		}
		h.Add(hf.Name, hf.Value)
	}
	return h, nil
}

func requestFromHeaders(decodeFn qpack.DecodeFunc, sizeLimit int, headerFields *[]qpack.HeaderField) (*http.Request, error) {
	hdr, err := parseHeaders(decodeFn, true, sizeLimit, headerFields)
	if err != nil {
		return nil, err
	}
	// concatenate cookie headers, see https://tools.ietf.org/html/rfc6265#section-5.4
	if len(hdr.Headers["Cookie"]) > 0 {
		hdr.Headers.Set("Cookie", strings.Join(hdr.Headers["Cookie"], "; "))
	}

	isConnect := hdr.Method == http.MethodConnect
	// Extended CONNECT, see https://datatracker.ietf.org/doc/html/rfc8441#section-4
	isExtendedConnected := isConnect && hdr.Protocol != ""
	if isExtendedConnected {
		if hdr.Scheme == "" || hdr.Path == "" || hdr.Authority == "" {
			return nil, errors.New("extended CONNECT: :scheme, :path and :authority must not be empty")
		}
	} else if isConnect {
		if hdr.Path != "" || hdr.Authority == "" { // normal CONNECT
			return nil, errors.New(":path must be empty and :authority must not be empty")
		}
	} else if len(hdr.Path) == 0 || len(hdr.Authority) == 0 || len(hdr.Method) == 0 {
		return nil, errors.New(":path, :authority and :method must not be empty")
	}

	if !isExtendedConnected && len(hdr.Protocol) > 0 {
		return nil, errors.New(":protocol must be empty")
	}

	var u *url.URL
	var requestURI string

	protocol := "HTTP/3.0"

	if isConnect {
		u = &url.URL{}
		if isExtendedConnected {
			u, err = url.ParseRequestURI(hdr.Path)
			if err != nil {
				return nil, err
			}
			protocol = hdr.Protocol
		} else {
			u.Path = hdr.Path
		}
		u.Scheme = hdr.Scheme
		u.Host = hdr.Authority
		requestURI = hdr.Authority
	} else {
		u, err = url.ParseRequestURI(hdr.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid content length: %w", err)
		}
		requestURI = hdr.Path
	}

	req := &http.Request{
		Method:        hdr.Method,
		URL:           u,
		Proto:         protocol,
		ProtoMajor:    3,
		ProtoMinor:    0,
		Header:        hdr.Headers,
		Body:          nil,
		ContentLength: hdr.ContentLength,
		Host:          hdr.Authority,
		RequestURI:    requestURI,
	}
	req.Trailer = extractAnnouncedTrailers(req.Header)
	return req, nil
}

// updateResponseFromHeaders sets up http.Response as an HTTP/3 response,
// using the decoded qpack header filed.
// It is only called for the HTTP header (and not the HTTP trailer).
// It takes an http.Response as an argument to allow the caller to set the trailer later on.
func updateResponseFromHeaders(rsp *http.Response, decodeFn qpack.DecodeFunc, sizeLimit int, headerFields *[]qpack.HeaderField) error {
	hdr, err := parseHeaders(decodeFn, false, sizeLimit, headerFields)
	if err != nil {
		return err
	}
	if hdr.Status == "" {
		return errors.New("missing :status field")
	}
	rsp.Proto = "HTTP/3.0"
	rsp.ProtoMajor = 3
	rsp.Header = hdr.Headers
	rsp.Trailer = extractAnnouncedTrailers(rsp.Header)
	rsp.ContentLength = hdr.ContentLength

	status, err := strconv.Atoi(hdr.Status)
	if err != nil {
		return fmt.Errorf("invalid status code: %w", err)
	}
	rsp.StatusCode = status
	rsp.Status = hdr.Status + " " + http.StatusText(status)
	return nil
}

// extractAnnouncedTrailers extracts trailer keys from the "Trailer" header.
// It returns a map with the announced keys set to nil values, and removes the "Trailer" header.
// It handles both duplicate as well as comma-separated values for the Trailer header.
// For example:
//
//	Trailer: Trailer1, Trailer2
//	Trailer: Trailer3
//
// Will result in a map containing the keys "Trailer1", "Trailer2", "Trailer3" with nil values.
func extractAnnouncedTrailers(header http.Header) http.Header {
	rawTrailers, ok := header["Trailer"]
	if !ok {
		return nil
	}

	trailers := make(http.Header)
	for _, rawVal := range rawTrailers {
		for _, val := range strings.Split(rawVal, ",") {
			trailers[http.CanonicalHeaderKey(textproto.TrimString(val))] = nil
		}
	}
	delete(header, "Trailer")
	return trailers
}

// writeTrailers encodes and writes HTTP trailers as a HEADERS frame.
// It returns true if trailers were written, false if there were no trailers to write.
func writeTrailers(wr io.Writer, trailers http.Header, streamID quic.StreamID, qlogger qlogwriter.Recorder) (bool, error) {
	var hasValues bool
	for k, vals := range trailers {
		if httpguts.ValidTrailerHeader(k) && len(vals) > 0 {
			hasValues = true
			break
		}
	}
	if !hasValues {
		return false, nil
	}

	var buf bytes.Buffer
	enc := qpack.NewEncoder(&buf)
	var headerFields []qlog.HeaderField
	if qlogger != nil {
		headerFields = make([]qlog.HeaderField, 0, len(trailers))
	}

	for k, vals := range trailers {
		if len(vals) == 0 {
			continue
		}
		if !httpguts.ValidTrailerHeader(k) {
			continue
		}
		lowercaseKey := strings.ToLower(k)
		for _, v := range vals {
			if err := enc.WriteField(qpack.HeaderField{Name: lowercaseKey, Value: v}); err != nil {
				return false, err
			}
			if qlogger != nil {
				headerFields = append(headerFields, qlog.HeaderField{Name: lowercaseKey, Value: v})
			}
		}
	}

	b := make([]byte, 0, frameHeaderLen+buf.Len())
	b = (&headersFrame{Length: uint64(buf.Len())}).Append(b)
	b = append(b, buf.Bytes()...)
	if qlogger != nil {
		qlogCreatedHeadersFrame(qlogger, streamID, len(b), buf.Len(), headerFields)
	}
	_, err := wr.Write(b)
	return true, err
}

func decodeTrailers(r io.Reader, hf *headersFrame, maxHeaderBytes int, decoder *qpack.Decoder, qlogger qlogwriter.Recorder, streamID quic.StreamID) (http.Header, error) {
	if hf.Length > uint64(maxHeaderBytes) {
		maybeQlogInvalidHeadersFrame(qlogger, streamID, hf.Length)
		return nil, fmt.Errorf("http3: HEADERS frame too large: %d bytes (max: %d)", hf.Length, maxHeaderBytes)
	}

	b := make([]byte, hf.Length)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	decodeFn := decoder.Decode(b)
	var fields []qpack.HeaderField
	if qlogger != nil {
		fields = make([]qpack.HeaderField, 0, 16)
	}
	trailers, err := parseTrailers(decodeFn, &fields)
	if err != nil {
		maybeQlogInvalidHeadersFrame(qlogger, streamID, hf.Length)
		return nil, err
	}
	if qlogger != nil {
		qlogParsedHeadersFrame(qlogger, streamID, hf, fields)
	}
	return trailers, nil
}
