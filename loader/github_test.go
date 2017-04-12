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

package loader

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGithub(t *testing.T) {
	path := "github.com/github/gitignore/Go.gitignore"
	name, loader, parts := pickLoader(path)
	assert.Equal(t, "github", name)
	assert.Equal(t, []string{"github", "gitignore", "Go.gitignore"}, parts)
	src, err := loader(path, parts)
	assert.NoError(t, err)
	assert.Equal(t, "https://raw.githubusercontent.com/github/gitignore/master/Go.gitignore", src)
	data, err := Load(afero.NewMemMapFs(), "/", path)
	if assert.NoError(t, err) {
		assert.Equal(t, path, data.Filename)
		assert.NotEmpty(t, data.Data)
	}
}
