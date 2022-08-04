package http

import (
	"fmt"
	"strings"
	"time"

	"go.k6.io/k6/js/common"
)

// FileData represents a binary file requiring multipart request encoding
type FileData struct {
	Data        []byte
	Filename    string
	ContentType string
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// File returns a FileData object.
func (mi *ModuleInstance) file(data interface{}, args ...string) FileData {
	// supply valid default if filename and content-type are not specified
	fname, ct := fmt.Sprintf("%d", time.Now().UnixNano()), "application/octet-stream"

	if len(args) > 0 {
		fname = args[0]

		if len(args) > 1 {
			ct = args[1]
		}
	}

	dt, err := common.ToBytes(data)
	if err != nil {
		common.Throw(mi.vu.Runtime(), err)
	}

	return FileData{
		Data:        dt,
		Filename:    fname,
		ContentType: ct,
	}
}
