package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		givenURL string
		want     string
	}{
		{
			name:     "empty_base_leaves_given",
			baseURL:  "",
			givenURL: "/foo",
			want:     "/foo",
		},
		{
			name:     "absolute_path",
			baseURL:  "http://example.com",
			givenURL: "/foo",
			want:     "http://example.com/foo",
		},
		{
			name:     "relative_to_directory_base",
			baseURL:  "http://example.com/bar/",
			givenURL: "foo",
			want:     "http://example.com/bar/foo",
		},
		{
			name:     "relative_replaces_last_segment",
			baseURL:  "http://example.com/bar",
			givenURL: "foo",
			want:     "http://example.com/foo",
		},
		{
			name:     "absolute_given_wins",
			baseURL:  "http://example.com",
			givenURL: "https://other.test/x",
			want:     "https://other.test/x",
		},
		{
			name:     "about_blank_is_absolute",
			baseURL:  "http://example.com",
			givenURL: "about:blank",
			want:     "about:blank",
		},
		{
			name:     "empty_given_is_the_base",
			baseURL:  "http://example.com/foo/",
			givenURL: "",
			want:     "http://example.com/foo/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ResolveURL(tt.baseURL, tt.givenURL))
		})
	}
}

func TestResolveURLPattern(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "*foo", ResolveURLPattern("http://example.com", "*foo"))
	assert.Empty(t, ResolveURLPattern("http://example.com", ""))
	assert.Equal(t, "http://example.com/api", ResolveURLPattern("http://example.com", "/api"))
}
