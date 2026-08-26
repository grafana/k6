// Package fetch implements utility functions for getting files from
// diverse sources
package fetch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

// FromURL returns the content from a url.
// Only http(s) and file scheme are supported.
// For files, the url must follow the schema file:///absolute/path/to/file
func FromURL(source string) ([]byte, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, err
	}

	var content io.ReadCloser
	switch u.Scheme {
	case "http", "https":
		//nolint: bodyclose  // is closed by means of content variable
		resp, err2 := http.Get(source)
		if err2 != nil {
			return nil, err2
		}
		content = resp.Body
	case "file":
		content, err = os.Open(u.Path)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported schema: %s", u.Scheme)
	}

	buf, err := io.ReadAll(content)
	_ = content.Close()

	return buf, err
}
