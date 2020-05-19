/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package fsext

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestTrimAferoPathSeparatorFs(t *testing.T) {
	m := afero.NewMemMapFs()
	fs := NewTrimFilePathSeparatorFs(m)
	expecteData := []byte("something")
	err := afero.WriteFile(fs, filepath.FromSlash("/path/to/somewhere"), expecteData, 0644)
	require.NoError(t, err)
	data, err := afero.ReadFile(m, "/path/to/somewhere")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, data)

	data, err = afero.ReadFile(m, "path/to/somewhere")
	require.NoError(t, err)
	require.Equal(t, expecteData, data)

	err = afero.WriteFile(fs, filepath.FromSlash("path/without/separtor"), expecteData, 0644)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}
