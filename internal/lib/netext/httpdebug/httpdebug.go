// Package httpdebug redacts credential headers in the --http-debug output. It is shared by the HTTP
// and gRPC debug logging.
package httpdebug

import (
	"bytes"
	"net/http"
	"strings"
)

// Mask replaces the redacted part of a header value.
const Mask = "***REDACTED***"

// IsSensitive reports whether the header (or gRPC metadata key) carries credentials.
func IsSensitive(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie",
		"x-api-key", "x-auth-token", "x-amz-security-token":
		return true
	default:
		return false
	}
}

// RedactValue masks the credential part of a sensitive header value and keeps the rest, so the
// output is still useful for debugging: the auth scheme, the cookie names and the Set-Cookie
// attributes stay visible. The caller has to check IsSensitive first, any other header is fully
// masked.
func RedactValue(name, value string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization":
		if scheme, _, found := strings.Cut(value, " "); found && isAuthScheme(scheme) {
			return scheme + " " + Mask
		}
		return Mask
	case "cookie":
		return redactCookie(value)
	case "set-cookie":
		return redactSetCookie(value)
	default:
		return Mask
	}
}

// The list from https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/Authentication#authentication_schemes
func isAuthScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "basic", "bearer", "digest", "hoba", "mutual", "negotiate", "ntlm", "vapid",
		"scram-sha-1", "scram-sha-256", "aws4-hmac-sha256":
		return true
	default:
		return false
	}
}

// A value that does not parse is fully masked, a broken cookie can still carry a secret.
func redactCookie(value string) string {
	cookies, err := http.ParseCookie(value)
	if err != nil {
		return Mask
	}
	pairs := make([]string, len(cookies))
	for i, c := range cookies {
		pairs[i] = c.Name + "=" + Mask
	}
	return strings.Join(pairs, "; ")
}

func redactSetCookie(value string) string {
	c, err := http.ParseSetCookie(value)
	if err != nil {
		return Mask
	}
	// keep the original attributes, Cookie.String() would normalize or drop some of them
	if _, attrs, found := strings.Cut(value, ";"); found {
		return c.Name + "=" + Mask + ";" + attrs
	}
	return c.Name + "=" + Mask
}

// RedactDump masks sensitive header values in an HTTP dump with "\n" line endings. Only the header
// block is touched, the start line and the body are kept as they are. The dump is returned as is
// when there is nothing to mask, and a new buffer is allocated only on the first masked header.
func RedactDump(dump []byte) []byte {
	head, _, _ := bytes.Cut(dump, []byte("\n\n"))
	var (
		out       []byte
		lineStart int
		copied    int
	)
	for line := range bytes.SplitSeq(head, []byte("\n")) {
		start := lineStart
		lineStart += len(line) + 1
		// the first line is the request or status line
		if start == 0 {
			continue
		}
		name, value, ok := bytes.Cut(line, []byte(":"))
		if !ok || !IsSensitive(string(name)) {
			continue
		}
		value = bytes.TrimSpace(value)
		if len(value) == 0 {
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(dump))
		}
		out = append(out, dump[copied:start]...)
		out = append(out, name...)
		out = append(out, ": "...)
		out = append(out, RedactValue(string(name), string(value))...)
		copied = start + len(line)
	}
	if out == nil {
		return dump
	}
	return append(out, dump[copied:]...)
}
