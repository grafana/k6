/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package encoding

import (
	"context"
	"encoding/base64"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/internal/modules"
)

func init() {
	modules.Register("k6/encoding", New())
}

type Encoding struct{}

func New() *Encoding {
	return &Encoding{}
}

func (e *Encoding) B64encode(ctx context.Context, input []byte, encoding string) string {
	switch encoding {
	case "rawstd":
		return base64.StdEncoding.WithPadding(base64.NoPadding).EncodeToString(input)
	case "std":
		return base64.StdEncoding.EncodeToString(input)
	case "rawurl":
		return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(input)
	case "url":
		return base64.URLEncoding.EncodeToString(input)
	default:
		return base64.StdEncoding.EncodeToString(input)
	}
}

func (e *Encoding) B64decode(ctx context.Context, input string, encoding string) string {
	var output []byte
	var err error

	switch encoding {
	case "rawstd":
		output, err = base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(input)
	case "std":
		output, err = base64.StdEncoding.DecodeString(input)
	case "rawurl":
		output, err = base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(input)
	case "url":
		output, err = base64.URLEncoding.DecodeString(input)
	default:
		output, err = base64.StdEncoding.DecodeString(input)
	}

	if err != nil {
		common.Throw(common.GetRuntime(ctx), err)
	}

	return string(output)
}
