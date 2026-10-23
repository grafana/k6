package httpdebug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   string
		cfg     Config
		unknown []string
	}{
		{value: "headers", cfg: Config{}},
		{value: "full", cfg: Config{Body: true}},
		{value: "unredacted", cfg: Config{Unredacted: true}},
		{value: "headers,unredacted", cfg: Config{Unredacted: true}},
		{value: "full,unredacted", cfg: Config{Body: true, Unredacted: true}},
		{value: "unredacted,full", cfg: Config{Body: true, Unredacted: true}},
		{value: " FULL , Unredacted ", cfg: Config{Body: true, Unredacted: true}},
		{value: "full,", cfg: Config{Body: true}},
		{value: "true", cfg: Config{}, unknown: []string{"true"}},
		{value: "full,unredact", cfg: Config{Body: true}, unknown: []string{"unredact"}},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			cfg, unknown := Parse(tt.value)
			assert.Equal(t, tt.cfg, cfg)
			assert.Equal(t, tt.unknown, unknown)
		})
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
		{"Proxy-Authorization", "Basic dXNlcjpwYXNz", "Basic ***REDACTED***"},
		{"Cookie", "session=abc; theme=dark", "session=***REDACTED***; theme=***REDACTED***"},
		{"Cookie", "lonely", "***REDACTED***"},
		{"Set-Cookie", "sid=abc; Path=/; HttpOnly", "sid=***REDACTED***; Path=/; HttpOnly"},
		{"Set-Cookie", "sid=abc", "sid=***REDACTED***"},
		{"X-Api-Key", "abc", "***REDACTED***"},
		{"X-Auth-Token", "abc", "***REDACTED***"},
		{"X-Amz-Security-Token", "abc", "***REDACTED***"},
		{"Authorization", "", ""},
		{"Content-Type", "text/plain", "text/plain"},
		{"WWW-Authenticate", `Digest realm="test"`, `Digest realm="test"`},
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
		dump := "GET /path HTTP/1.1\r\n" +
			"Host: example.com\r\n" +
			"Authorization: Bearer secret\r\n" +
			"Cookie: a=1; b=2\r\n" +
			"\r\n" +
			"Authorization: Bearer body-is-not-touched"
		expected := "GET /path HTTP/1.1\r\n" +
			"Host: example.com\r\n" +
			"Authorization: Bearer ***REDACTED***\r\n" +
			"Cookie: a=***REDACTED***; b=***REDACTED***\r\n" +
			"\r\n" +
			"Authorization: Bearer body-is-not-touched"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})

	t.Run("response with multiple Set-Cookie", func(t *testing.T) {
		t.Parallel()
		dump := "HTTP/1.1 200 OK\r\n" +
			"Set-Cookie: a=1; Path=/\r\n" +
			"Set-Cookie: b=2; HttpOnly\r\n" +
			"Content-Length: 0\r\n" +
			"\r\n"
		expected := "HTTP/1.1 200 OK\r\n" +
			"Set-Cookie: a=***REDACTED***; Path=/\r\n" +
			"Set-Cookie: b=***REDACTED***; HttpOnly\r\n" +
			"Content-Length: 0\r\n" +
			"\r\n"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})

	t.Run("no header terminator", func(t *testing.T) {
		t.Parallel()
		dump := "HTTP/1.1 200 OK\r\nX-Api-Key: secret"
		expected := "HTTP/1.1 200 OK\r\nX-Api-Key: ***REDACTED***"
		assert.Equal(t, expected, string(RedactDump([]byte(dump))))
	})
}
