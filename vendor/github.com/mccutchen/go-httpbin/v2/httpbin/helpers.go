package httpbin

import (
	"bytes"
	crypto_rand "crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// requestHeaders takes in incoming request and returns an http.Header map
// suitable for inclusion in our response data structures.
//
// This is necessary to ensure that the incoming Host and Transfer-Encoding
// headers are included, because golang only exposes those values on the
// http.Request struct itself.
func getRequestHeaders(r *http.Request, fn headersProcessorFunc) http.Header {
	h := r.Header
	h.Set("Host", r.Host)
	if len(r.TransferEncoding) > 0 {
		h.Set("Transfer-Encoding", strings.Join(r.TransferEncoding, ","))
	}
	if fn != nil {
		return fn(h)
	}
	return h
}

// getClientIP tries to get a reasonable value for the IP address of the
// client making the request. Note that this value will likely be trivial to
// spoof, so do not rely on it for security purposes.
func getClientIP(r *http.Request) string {
	// Special case some hosting platforms that provide the value directly.
	if clientIP := r.Header.Get("Fly-Client-IP"); clientIP != "" {
		return clientIP
	}
	if clientIP := r.Header.Get("CF-Connecting-IP"); clientIP != "" {
		return clientIP
	}
	if clientIP := r.Header.Get("Fastly-Client-IP"); clientIP != "" {
		return clientIP
	}
	if clientIP := r.Header.Get("True-Client-IP"); clientIP != "" {
		return clientIP
	}

	// Try to pull a reasonable value from the X-Forwarded-For header, if
	// present, by taking the first entry in a comma-separated list of IPs.
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return strings.TrimSpace(strings.SplitN(forwardedFor, ",", 2)[0])
	}

	// Finally, fall back on the actual remote addr from the request.
	remoteAddr := r.RemoteAddr
	if strings.IndexByte(remoteAddr, ':') > 0 {
		ip, _, _ := net.SplitHostPort(remoteAddr)
		return ip
	}
	return remoteAddr
}

func getURL(r *http.Request) *url.URL {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = r.Header.Get("X-Forwarded-Protocol")
	}
	if scheme == "" && r.Header.Get("X-Forwarded-Ssl") == "on" {
		scheme = "https"
	}
	if scheme == "" && r.TLS != nil {
		scheme = "https"
	}
	if scheme == "" {
		scheme = "http"
	}

	host := r.URL.Host
	if host == "" {
		host = r.Host
	}

	return &url.URL{
		Scheme:     scheme,
		Opaque:     r.URL.Opaque,
		User:       r.URL.User,
		Host:       host,
		Path:       r.URL.Path,
		RawPath:    r.URL.RawPath,
		ForceQuery: r.URL.ForceQuery,
		RawQuery:   r.URL.RawQuery,
		Fragment:   r.URL.Fragment,
	}
}

func writeResponse(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(body)
}

func mustMarshalJSON(w io.Writer, val any) {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(val); err != nil {
		panic(err.Error())
	}
}

func writeJSON(status int, w http.ResponseWriter, val any) {
	w.Header().Set("Content-Type", jsonContentType)
	w.WriteHeader(status)
	mustMarshalJSON(w, val)
}

func writeHTML(w http.ResponseWriter, body []byte, status int) {
	writeResponse(w, status, htmlContentType, body)
}

func writeError(w http.ResponseWriter, code int, err error) {
	resp := errorRespnose{
		Error:      http.StatusText(code),
		StatusCode: code,
	}
	if err != nil {
		resp.Detail = err.Error()
	}
	writeJSON(code, w, resp)
}

// parseFiles handles reading the contents of files in a multipart FileHeader
// and returning a map that can be used as the Files attribute of a response
func parseFiles(fileHeaders map[string][]*multipart.FileHeader) (map[string][]string, error) {
	files := map[string][]string{}
	for k, fs := range fileHeaders {
		files[k] = []string{}

		for _, f := range fs {
			fh, err := f.Open()
			if err != nil {
				return nil, err
			}
			contents, err := io.ReadAll(fh)
			if err != nil {
				return nil, err
			}
			files[k] = append(files[k], string(contents))
		}
	}
	return files, nil
}

// parseBody handles parsing a request body into our standard API response,
// taking care to only consume the request body once based on the Content-Type
// of the request. The given bodyResponse will be modified.
//
// Note: this function expects callers to limit the the maximum size of the
// request body. See, e.g., the limitRequestSize middleware.
func parseBody(r *http.Request, resp *bodyResponse) error {
	defer r.Body.Close()

	// Always set resp.Data to the incoming request body, in case we don't know
	// how to handle the content type
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// After reading the body to populate resp.Data, we need to re-wrap it in
	// an io.Reader for further processing below
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// if we read an empty body, there's no need to do anything further
	if len(body) == 0 {
		return nil
	}

	// Always store the "raw" incoming request body
	resp.Data = string(body)

	contentType, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")

	switch contentType {
	case "text/html", "text/plain":
		// no need for extra parsing, string body is already set above
		return nil

	case "application/x-www-form-urlencoded":
		// r.ParseForm() does not populate r.PostForm for DELETE or GET
		// requests, but we need it to for compatibility with the httpbin
		// implementation, so we trick it with this ugly hack.
		if r.Method == http.MethodDelete || r.Method == http.MethodGet {
			originalMethod := r.Method
			r.Method = http.MethodPost
			defer func() { r.Method = originalMethod }()
		}
		if err := r.ParseForm(); err != nil {
			return err
		}
		resp.Form = r.PostForm

	case "multipart/form-data":
		// The memory limit here only restricts how many parts will be kept in
		// memory before overflowing to disk:
		// https://golang.org/pkg/net/http/#Request.ParseMultipartForm
		if err := r.ParseMultipartForm(1024); err != nil {
			return err
		}
		resp.Form = r.PostForm
		files, err := parseFiles(r.MultipartForm.File)
		if err != nil {
			return err
		}
		resp.Files = files

	case "application/json":
		if err := json.NewDecoder(r.Body).Decode(&resp.JSON); err != nil {
			return err
		}

	default:
		// If we don't have a special case for the content type, return it
		// encoded as base64 data url
		resp.Data = encodeData(body, contentType)
	}

	return nil
}

// return provided string as base64 encoded data url, with the given content type
func encodeData(body []byte, contentType string) string {
	// If no content type is provided, default to application/octet-stream
	if contentType == "" {
		contentType = binaryContentType
	}
	data := base64.URLEncoding.EncodeToString(body)
	return string("data:" + contentType + ";base64," + data)
}

func parseStatusCode(input string) (int, error) {
	return parseBoundedStatusCode(input, 100, 599)
}

func parseBoundedStatusCode(input string, minVal, maxVal int) (int, error) {
	code, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid status code: %q: %w", input, err)
	}
	if code < minVal || code > maxVal {
		return 0, fmt.Errorf("invalid status code: %d not in range [%d, %d]", code, minVal, maxVal)
	}
	return code, nil
}

// parseDuration takes a user's input as a string and attempts to convert it
// into a time.Duration. If not given as a go-style duration string, the input
// is assumed to be seconds as a float.
func parseDuration(input string) (time.Duration, error) {
	d, err := time.ParseDuration(input)
	if err != nil {
		n, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return 0, err
		}
		d = time.Duration(n*1000) * time.Millisecond
	}
	return d, nil
}

// parseBoundedDuration parses a time.Duration from user input and ensures that
// it is within a given maximum and minimum time
func parseBoundedDuration(input string, minVal, maxVal time.Duration) (time.Duration, error) {
	d, err := parseDuration(input)
	if err != nil {
		return 0, err
	}

	if d > maxVal {
		err = fmt.Errorf("duration %s longer than %s", d, maxVal)
	} else if d < minVal {
		err = fmt.Errorf("duration %s shorter than %s", d, minVal)
	}
	return d, err
}

// Returns a new rand.Rand from the given seed string.
func parseSeed(rawSeed string) (*rand.Rand, error) {
	var seed int64
	if rawSeed != "" {
		var err error
		seed, err = strconv.ParseInt(rawSeed, 10, 64)
		if err != nil {
			return nil, err
		}
	} else {
		seed = time.Now().UnixNano()
	}

	src := rand.NewSource(seed)
	rng := rand.New(src)
	return rng, nil
}

func computePausePerWrite(duration time.Duration, count int64) time.Duration {
	pause := duration
	if count > 1 {
		// compensate for lack of pause after final write (i.e. if we're
		// doing 10 writes we will only pause 9 times).
		//
		// note: use ceiling division to ensure pause*count >= duration when
		// count does not divided evenly.
		n := time.Duration(count - 1)
		pause = (duration + n - 1) / n
	}
	return pause
}

// syntheticByteStream implements the ReadSeeker interface to allow reading
// arbitrary subsets of bytes up to a maximum size given a function for
// generating the byte at a given offset.
type syntheticByteStream struct {
	mu sync.Mutex

	size         int64
	factory      func(int64) byte
	pausePerByte time.Duration

	// internal offset for tracking the current position in the stream
	offset int64
}

// newSyntheticByteStream returns a new stream of bytes of a specific size,
// given a factory function for generating the byte at a given offset.
func newSyntheticByteStream(size int64, duration time.Duration, factory func(int64) byte) io.ReadSeeker {
	return &syntheticByteStream{
		size:         size,
		pausePerByte: duration / time.Duration(size),
		factory:      factory,
	}
}

// Read implements the Reader interface for syntheticByteStream
func (s *syntheticByteStream) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := s.offset
	end := start + int64(len(p))
	var err error
	if end >= s.size {
		err = io.EOF
		end = s.size
	}

	for idx := start; idx < end; idx++ {
		p[idx-start] = s.factory(idx)
	}
	s.offset = end

	if s.pausePerByte > 0 {
		time.Sleep(s.pausePerByte * time.Duration(end-start))
	}

	return int(end - start), err
}

// Seek implements the Seeker interface for syntheticByteStream
func (s *syntheticByteStream) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch whence {
	case io.SeekStart:
		s.offset = offset
	case io.SeekCurrent:
		s.offset += offset
	case io.SeekEnd:
		s.offset = s.size - offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}

	if s.offset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}

	return s.offset, nil
}

func sha1hash(input string) string {
	h := sha1.New()
	return fmt.Sprintf("%x", h.Sum([]byte(input)))
}

func uuidv4() string {
	buff := make([]byte, 16)
	if _, err := crypto_rand.Read(buff[:]); err != nil {
		panic(err)
	}
	buff[6] = (buff[6] & 0x0f) | 0x40 // Version 4
	buff[8] = (buff[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", buff[0:4], buff[4:6], buff[6:8], buff[8:10], buff[10:])
}

// base64Helper encapsulates a base64 operation (encode or decode) and its input
// data.
type base64Helper struct {
	maxLen    int64
	operation string
	data      string
}

// newBase64Helper creates a new base64Helper from a URL path, which should be
// in one of two forms:
// - /base64/<base64_encoded_data>
// - /base64/<operation>/<base64_encoded_data>
func newBase64Helper(r *http.Request, maxLen int64) *base64Helper {
	b := &base64Helper{
		operation: r.PathValue("operation"),
		data:      r.PathValue("data"),
		maxLen:    maxLen,
	}
	if b.operation == "" {
		b.operation = "decode"
	}
	return b
}

// transform performs the base64 operation on the input data.
func (b *base64Helper) transform() ([]byte, error) {
	if dataLen := int64(len(b.data)); dataLen == 0 {
		return nil, errors.New("no input data")
	} else if dataLen > b.maxLen {
		return nil, fmt.Errorf("input data exceeds max length of %d", b.maxLen)
	}

	switch b.operation {
	case "encode":
		return b.encode(), nil
	case "decode":
		result, err := b.decode()
		if err != nil {
			return nil, fmt.Errorf("base64 decode failed: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid operation: %s", b.operation)
	}
}

func (b *base64Helper) encode() []byte {
	// always encode using the URL-safe character set
	buff := make([]byte, base64.URLEncoding.EncodedLen(len(b.data)))
	base64.URLEncoding.Encode(buff, []byte(b.data))
	return buff
}

func (b *base64Helper) decode() ([]byte, error) {
	// first, try URL-safe encoding, then std encoding
	if result, err := base64.URLEncoding.DecodeString(b.data); err == nil {
		return result, nil
	}
	return base64.StdEncoding.DecodeString(b.data)
}

func wildCardToRegexp(pattern string) string {
	components := strings.Split(pattern, "*")
	if len(components) == 1 {
		// if len is 1, there are no *'s, return exact match pattern
		return "^" + pattern + "$"
	}
	var result strings.Builder
	for i, literal := range components {

		// Replace * with .*
		if i > 0 {
			result.WriteString(".*")
		}

		// Quote any regular expression meta characters in the
		// literal text.
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return "^" + result.String() + "$"
}

func createExcludeHeadersProcessor(excludeRegex *regexp.Regexp) headersProcessorFunc {
	return func(headers http.Header) http.Header {
		result := make(http.Header)
		for k, v := range headers {
			matched := excludeRegex.Match([]byte(k))
			if matched {
				continue
			}
			result[k] = v
		}

		return result
	}
}

func createFullExcludeRegex(excludeHeaders string) *regexp.Regexp {
	// comma separated list of headers to exclude from response
	tmp := strings.Split(excludeHeaders, ",")

	tmpRegexStrings := make([]string, 0)
	for _, v := range tmp {
		s := strings.TrimSpace(v)
		if len(s) == 0 {
			continue
		}
		pattern := wildCardToRegexp(s)
		tmpRegexStrings = append(tmpRegexStrings, pattern)
	}

	if len(tmpRegexStrings) > 0 {
		tmpRegexStr := strings.Join(tmpRegexStrings, "|")
		result := regexp.MustCompile("(?i)" + "(" + tmpRegexStr + ")")
		return result
	}

	return nil
}

// weightedChoice represents a choice with its associated weight.
type weightedChoice[T any] struct {
	Choice T
	Weight float64
}

// parseWeighteChoices parses a comma-separated list of choices in
// choice:weight format, where weight is an optional floating point number.
func parseWeightedChoices[T any](rawChoices string, parser func(string) (T, error)) ([]weightedChoice[T], error) {
	if rawChoices == "" {
		return nil, nil
	}

	var (
		choicePairs = strings.Split(rawChoices, ",")
		choices     = make([]weightedChoice[T], 0, len(choicePairs))
		err         error
	)
	for _, choicePair := range choicePairs {
		weight := 1.0
		rawChoice, rawWeight, found := strings.Cut(choicePair, ":")
		if found {
			weight, err = strconv.ParseFloat(rawWeight, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid weight value: %q", rawWeight)
			}
		}
		choice, err := parser(rawChoice)
		if err != nil {
			return nil, fmt.Errorf("invalid choice value: %q", rawChoice)
		}
		choices = append(choices, weightedChoice[T]{Choice: choice, Weight: weight})
	}
	return choices, nil
}

// weightedRandomChoice returns a randomly chosen element from the weighted
// choices, given as a slice of "choice:weight" strings where weight is a
// floating point number. Weights do not need to sum to 1.
func weightedRandomChoice[T any](choices []weightedChoice[T], randomFloat64 func() float64) T {
	// Calculate total weight
	var totalWeight float64
	for _, wc := range choices {
		totalWeight += wc.Weight
	}
	randomNumber := randomFloat64() * totalWeight
	currentWeight := 0.0
	for _, wc := range choices {
		currentWeight += wc.Weight
		if randomNumber < currentWeight {
			return wc.Choice
		}
	}
	panic("failed to select a weighted random choice")
}

// Server-Timing header/trailer helpers. See MDN docs for reference:
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Server-Timing
type serverTiming struct {
	name string
	dur  time.Duration
	desc string
}

func encodeServerTimings(timings []serverTiming) string {
	entries := make([]string, len(timings))
	for i, t := range timings {
		ms := t.dur.Seconds() * 1e3
		entries[i] = fmt.Sprintf("%s;dur=%0.2f;desc=\"%s\"", t.name, ms, t.desc)
	}
	return strings.Join(entries, ", ")
}

// The following content types are considered safe enough to skip HTML-escaping
// response bodies.
//
// See [1] for an example of the wide variety of unsafe content types, which
// varies by browser vendor and could change in the future.
//
// [1]: https://github.com/BlackFan/content-type-research/blob/4e4347254/XSS.md
var safeContentTypes = map[string]bool{
	"text/plain":               true,
	"application/json":         true,
	"application/octet-string": true,
}

// isDangerousContentType determines whether the given Content-Type header
// value could be unsafe (e.g. at risk of XSS) when rendered by a web browser.
func isDangerousContentType(ct string) bool {
	mediatype, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return true
	}
	return !safeContentTypes[mediatype]
}
