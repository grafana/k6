// Package httpdebug holds the parsing of the --http-debug option and the redaction of credential
// headers in the debug output. It is shared by the HTTP and gRPC debug logging.
package httpdebug

import (
	"bytes"
	"strings"
)

// Mask replaces the redacted part of a header value.
const Mask = "***REDACTED***"

// Config is the parsed value of the --http-debug option.
type Config struct {
	// Body enables dumping of request and response bodies ("full").
	Body bool
	// Unredacted disables masking of credential headers ("unredacted").
	Unredacted bool
}

// Parse parses a comma-separated --http-debug value like "full,unredacted". Tokens can come in any
// order. Unknown tokens are returned separately so the caller can warn about them, they do not
// change the config - any non-empty value keeps meaning "headers" as before.
func Parse(s string) (Config, []string) {
	var (
		cfg     Config
		unknown []string
	)
	for token := range strings.SplitSeq(s, ",") {
		token = strings.TrimSpace(token)
		switch strings.ToLower(token) {
		case "", "headers":
		case "full":
			cfg.Body = true
		case "unredacted":
			cfg.Unredacted = true
		default:
			unknown = append(unknown, token)
		}
	}
	return cfg, unknown
}

//nolint:gochecknoglobals
var sensitiveHeaders = map[string]struct{}{
	"authorization":        {},
	"proxy-authorization":  {},
	"cookie":               {},
	"set-cookie":           {},
	"x-api-key":            {},
	"x-auth-token":         {},
	"x-amz-security-token": {},
}

// IsSensitive reports whether the header (or gRPC metadata key) carries credentials.
func IsSensitive(name string) bool {
	_, ok := sensitiveHeaders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// RedactValue masks the credential part of a sensitive header value and keeps the rest, so the
// output is still useful for debugging: the auth scheme, the cookie names and the Set-Cookie
// attributes stay visible. Values of non-sensitive headers are returned as is.
func RedactValue(name, value string) string {
	if !IsSensitive(name) || value == "" {
		return value
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization":
		if scheme, _, found := strings.Cut(value, " "); found {
			return scheme + " " + Mask
		}
		return Mask
	case "cookie":
		pairs := strings.Split(value, ";")
		for i, pair := range pairs {
			pairs[i] = redactCookiePair(pair)
		}
		return strings.Join(pairs, ";")
	case "set-cookie":
		pair, attrs, found := strings.Cut(value, ";")
		if found {
			return redactCookiePair(pair) + ";" + attrs
		}
		return redactCookiePair(pair)
	default:
		return Mask
	}
}

func redactCookiePair(pair string) string {
	name, _, found := strings.Cut(pair, "=")
	if !found {
		return Mask
	}
	return name + "=" + Mask
}

// RedactDump masks sensitive header values in an HTTP dump produced by httputil. Only the header
// block is touched, the start line and the body are kept as they are.
func RedactDump(dump []byte) []byte {
	head, body, found := bytes.Cut(dump, []byte("\r\n\r\n"))
	lines := bytes.Split(head, []byte("\r\n"))
	// the first line is the request or status line
	for i := 1; i < len(lines); i++ {
		name, value, ok := bytes.Cut(lines[i], []byte(":"))
		if !ok || !IsSensitive(string(name)) {
			continue
		}
		redacted := RedactValue(string(name), strings.TrimSpace(string(value)))
		lines[i] = []byte(string(name) + ": " + redacted)
	}

	out := bytes.Join(lines, []byte("\r\n"))
	if found {
		out = append(out, "\r\n\r\n"...)
		out = append(out, body...)
	}
	return out
}
