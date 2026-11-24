package httpdebug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSensitive(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"Authorization", "proxy-authorization", "Cookie", "Set-Cookie",
		"X-Api-Key", "X-Auth-Token", "X-Amz-Security-Token", " authorization ",
	} {
		assert.True(t, IsSensitive(name), name)
	}
	for _, name := range []string{"Content-Type", "WWW-Authenticate", ""} {
		assert.False(t, IsSensitive(name), name)
	}
}

func TestRedactValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, value, expected string
	}{
		{"Authorization", "Bearer abc.def", "Bearer ***REDACTED***"},
		{"authorization", "Basic dXNlcjpwYXNz", "Basic ***REDACTED***"},
		{"Authorization", "abc", "***REDACTED***"},
		{"Authorization", "bearer abc", "bearer ***REDACTED***"},
		{"Authorization", "rawsecret extra", "***REDACTED***"},
		{"Authorization", "SCRAM-SHA-256 data=abc", "SCRAM-SHA-256 ***REDACTED***"},
		{"Authorization", "Digest username=\"u\", response=\"r\"", "Digest ***REDACTED***"},
		{"Proxy-Authorization", "Basic dXNlcjpwYXNz", "Basic ***REDACTED***"},
		{"Cookie", "session=abc; theme=dark", "session=***REDACTED***; theme=***REDACTED***"},
		{"Cookie", "lonely", "***REDACTED***"},
		{"Cookie", "a=1; broken", "***REDACTED***"},
		{"Set-Cookie", "sid=abc; Path=/; HttpOnly", "sid=***REDACTED***; Path=/; HttpOnly"},
		{"Set-Cookie", "sid=abc", "sid=***REDACTED***"},
		{"Set-Cookie", "=abc", "***REDACTED***"},
		{"X-Api-Key", "abc", "***REDACTED***"},
		{"X-Auth-Token", "abc", "***REDACTED***"},
		{"X-Amz-Security-Token", "abc", "***REDACTED***"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.value, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, RedactValue(tt.name, tt.value))
		})
	}
}

func TestRedactDump(t *testing.T) {
	t.Parallel()

	t.Run("request", func(t *testing.T) {
		t.Parallel()
		dump := "GET /path HTTP/1.1\n" +
			"Host: example.com\n" +
			"Authorization: Bearer secret\n" +
			"Cookie: a=1; b=2\n" +
			"\n" +
			"Authorization: Bearer body-is-not-touched"
		expected := "GET /path HTTP/1.1\n" +
			"Host: example.com\n" +
			"Authorization: Bearer ***REDACTED***\n" +
			"Cookie: a=***REDACTED***; b=***REDACTED***\n" +
			"\n" +
			"Authorization: Bearer body-is-not-touched"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})

	t.Run("response with multiple Set-Cookie", func(t *testing.T) {
		t.Parallel()
		dump := "HTTP/1.1 200 OK\n" +
			"Set-Cookie: a=1; Path=/\n" +
			"Set-Cookie: b=2; HttpOnly\n" +
			"Content-Length: 0\n" +
			"\n"
		expected := "HTTP/1.1 200 OK\n" +
			"Set-Cookie: a=***REDACTED***; Path=/\n" +
			"Set-Cookie: b=***REDACTED***; HttpOnly\n" +
			"Content-Length: 0\n" +
			"\n"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})

	t.Run("no header terminator", func(t *testing.T) {
		t.Parallel()
		dump := "HTTP/1.1 200 OK\nX-Api-Key: secret"
		expected := "HTTP/1.1 200 OK\nX-Api-Key: ***REDACTED***"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})

	t.Run("nothing to redact returns the original", func(t *testing.T) {
		t.Parallel()
		dump := []byte("HTTP/1.1 200 OK\nContent-Type: text/plain\nAuthorization:\n\nbody")
		out := RedactDump(dump)
		assert.Equal(t, string(dump), string(out))
		assert.Same(t, &dump[0], &out[0])
	})
}
