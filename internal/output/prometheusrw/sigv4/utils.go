package sigv4

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

func buildAwsNoEscape() [256]bool {
	var noEscape [256]bool

	for i := 0; i < len(noEscape); i++ {
		// AWS expects every character except these to be escaped
		noEscape[i] = (i >= 'A' && i <= 'Z') ||
			(i >= 'a' && i <= 'z') ||
			(i >= '0' && i <= '9') ||
			i == '-' ||
			i == '.' ||
			i == '_' ||
			i == '~' ||
			i == '/'
	}
	return noEscape
}

// escapePath escapes part of a URL path in Amazon style.
// except for the noEscape provided.
// inspired by github.com/aws/smithy-go/encoding/httpbinding EscapePath method
func escapePath(path string, noEscape [256]bool) string {
	var buf bytes.Buffer
	for i := 0; i < len(path); i++ {
		c := path[i]
		if noEscape[c] {
			buf.WriteByte(c)
			continue
		}
		fmt.Fprintf(&buf, "%%%02X", c)
	}
	return buf.String()
}

// stripExcessSpaces will remove the leading and trailing spaces, and side-by-side spaces are converted
// into a single space.
func stripExcessSpaces(str string) string {
	if !strings.Contains(str, "  ") && !strings.Contains(str, "\t") {
		return str
	}

	builder := strings.Builder{}
	lastFoundSpace := -1
	const space = ' '
	str = strings.TrimSpace(str)
	for i := 0; i < len(str); i++ {
		if str[i] == space || str[i] == '\t' {
			lastFoundSpace = i
			continue
		}

		if lastFoundSpace > 0 && builder.Len() != 0 {
			builder.WriteByte(space)
		}
		builder.WriteByte(str[i])
		lastFoundSpace = -1
	}
	return builder.String()
}

// getURIPath returns the escaped URI component from the provided URL.
// Ported from inspired by github.com/aws/aws-sdk-go-v2/aws/signer/internal/v4 GetURIPath
func getURIPath(u *url.URL) string {
	var uriPath string

	opaque := u.Opaque
	if len(opaque) == 0 {
		uriPath = u.EscapedPath()
	}

	if len(opaque) == 0 && len(uriPath) == 0 {
		return "/"
	}

	const schemeSep, pathSep, queryStart = "//", "/", "?"

	// Cutout the scheme separator if present.
	if strings.Index(opaque, schemeSep) == 0 {
		opaque = opaque[len(schemeSep):]
	}

	// Cut off the query string if present.
	if idx := strings.Index(opaque, queryStart); idx >= 0 {
		opaque = opaque[:idx]
	}

	// capture URI path starting with first path separator.
	if idx := strings.Index(opaque, pathSep); idx >= 0 {
		uriPath = opaque[idx:]
	}

	if len(uriPath) == 0 {
		uriPath = "/"
	}

	return uriPath
}
