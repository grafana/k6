/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package http

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/loadimpact/k6/js/common"
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

// File returns a FileData parameter
func (h *HTTP) File(ctx context.Context, data interface{}, args ...string) FileData {
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
		common.Throw(common.GetRuntime(ctx), err)
	}

	return FileData{
		Data:        dt,
		Filename:    fname,
		ContentType: ct,
	}
}
