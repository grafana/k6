/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package v1

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func newRequestWithEngine(engine *lib.Engine, method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	return r.WithContext(common.WithEngine(r.Context(), engine))
}

func TestNewHandler(t *testing.T) {
	assert.NotNil(t, NewHandler())
}
