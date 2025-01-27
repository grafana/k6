package sigv4

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripExcessSpaces(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		arg  string
		want string
	}{
		{
			arg:  `AWS4-HMAC-SHA256 Credential=AKIDFAKEIDFAKEID/20160628/us-west-2/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=1234567890abcdef1234567890abcdef1234567890abcdef`,
			want: `AWS4-HMAC-SHA256 Credential=AKIDFAKEIDFAKEID/20160628/us-west-2/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=1234567890abcdef1234567890abcdef1234567890abcdef`,
		},
		{
			arg:  "a   b   c   d",
			want: "a b c d",
		},
		{
			arg:  "   abc   def   ghi   jk   ",
			want: "abc def ghi jk",
		},
		{
			arg:  "   123    456    789          101112   ",
			want: "123 456 789 101112",
		},
		{
			arg:  "12     3       1abc123",
			want: "12 3 1abc123",
		},
		{
			arg:  "aaa \t bb",
			want: "aaa bb",
		},
	}

	for _, tc := range testcases {
		assert.Equal(t, tc.want, stripExcessSpaces(tc.arg))
	}
}

func TestGetUriPath(t *testing.T) {
	t.Parallel()

	testcases := map[string]struct {
		arg  string
		want string
	}{
		"schema and port": {
			arg:  "https://localhost:9000",
			want: "/",
		},
		"schema and no port": {
			arg:  "https://localhost",
			want: "/",
		},
		"no schema": {
			arg:  "localhost:9000",
			want: "/",
		},
		"no schema + path": {
			arg:  "localhost:9000/abc123",
			want: "/abc123",
		},
		"no schema, with separator": {
			arg:  "//localhost:9000",
			want: "/",
		},
		"no scheme, no port, with separator": {
			arg:  "//localhost",
			want: "/",
		},
		"no scheme, with separator, with path": {
			arg:  "//localhost:9000/abc123",
			want: "/abc123",
		},
		"no scheme, no port, with separator, with path": {
			arg:  "//localhost/abc123",
			want: "/abc123",
		},
		"no schema, query string": {
			arg:  "localhost:9000/abc123?efg=456",
			want: "/abc123",
		},
	}
	for name, tc := range testcases {
		u, err := url.Parse(tc.arg)
		if err != nil {
			t.Fatal(err)
		}

		got := getURIPath(u)
		if tc.want != got {
			t.Fatalf("test %v failed, want %v got %v \n", name, tc.want, got)
		}
	}
}

func TestGetUriPath_invalid_url_noescape(t *testing.T) {
	t.Parallel()

	arg := &url.URL{
		Opaque: "//example.org/bucket/key-._~,!@#$%^&*()",
	}

	want := "/bucket/key-._~,!@#$%^&*()"
	got := getURIPath(arg)
	assert.Equal(t, want, got)
}

func TestEscapePath(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		arg  string
		want string
	}{
		{
			arg:  "/",
			want: "/",
		},
		{
			arg:  "/abc",
			want: "/abc",
		},
		{
			arg:  "/abc129",
			want: "/abc129",
		},
		{
			arg:  "/abc-def",
			want: "/abc-def",
		},
		{
			arg:  "/abc.xyz~123-456",
			want: "/abc.xyz~123-456",
		},
		{
			arg:  "/abc def-ghi",
			want: "/abc%20def-ghi",
		},
		{
			arg:  "abc!def ghi",
			want: "abc%21def%20ghi",
		},
	}

	noEscape := buildAwsNoEscape()

	for _, tc := range testcases {
		assert.Equal(t, tc.want, escapePath(tc.arg, noEscape))
	}
}
