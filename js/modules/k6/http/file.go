package http

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/sobek"
)

// FileData represents a binary file requiring multipart request encoding
type FileData struct {
	Data        any
	Filename    string
	ContentType string
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"") //nolint:gochecknoglobals

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// File returns a FileData object.
func (mi *ModuleInstance) file(data any, args ...string) (*FileData, error) {
	// supply valid default if filename and content-type are not specified
	fname, ct := fmt.Sprintf("%d", time.Now().UnixNano()), "application/octet-stream"

	if len(args) > 0 {
		fname = args[0]

		if len(args) > 1 {
			ct = args[1]
		}
	}

	switch data.(type) {
	case string, sobek.ArrayBuffer:
	default:
		return nil, fmt.Errorf("invalid type %T, expected string or ArrayBuffer", data)
	}

	return &FileData{
		Data:        data,
		Filename:    fname,
		ContentType: ct,
	}, nil
}
