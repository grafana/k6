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

package json

import (
	"os"
	"testing"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	testdata := map[string]bool{
		"/nonexistent/badplacetolog.log": false,
		"./okplacetolog.log":             true,
		"okplacetolog.log":               true,
	}

	for path, succ := range testdata {
		path, succ := path, succ
		t.Run("path="+path, func(t *testing.T) {
			defer func() { _ = os.Remove(path) }()

			collector, err := New(testutils.NewLogger(t), afero.NewOsFs(), path)
			if succ {
				assert.NoError(t, err)
				assert.NotNil(t, collector)
			} else {
				assert.Error(t, err)
				assert.Nil(t, collector)
			}
		})
	}
}
