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

func TestCDNJS(t *testing.T) {
	paths := map[string]struct {
		parts []string
		src   string
	}{
		"cdnjs.com/libraries/Faker": {
			[]string{"Faker", "", ""},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/[\d\.]+/faker.min.js$`,
		},
		"cdnjs.com/libraries/Faker/faker.js": {
			[]string{"Faker", "", "faker.js"},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/[\d\.]+/faker.js$`,
		},
		"cdnjs.com/libraries/Faker/locales/en_AU/faker.en_AU.min.js": {
			[]string{"Faker", "", "locales/en_AU/faker.en_AU.min.js"},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/[\d\.]+/locales/en_AU/faker.en_AU.min.js$`,
		},
		"cdnjs.com/libraries/Faker/3.1.0": {
			[]string{"Faker", "3.1.0", ""},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/faker.min.js$`,
		},
		"cdnjs.com/libraries/Faker/3.1.0/faker.js": {
			[]string{"Faker", "3.1.0", "faker.js"},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/faker.js$`,
		},
		"cdnjs.com/libraries/Faker/3.1.0/locales/en_AU/faker.en_AU.min.js": {
			[]string{"Faker", "3.1.0", "locales/en_AU/faker.en_AU.min.js"},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/locales/en_AU/faker.en_AU.min.js$`,
		},
		"cdnjs.com/libraries/Faker/0.7.2": {
			[]string{"Faker", "0.7.2", ""},
			`^https://cdnjs.cloudflare.com/ajax/libs/Faker/0.7.2/MinFaker.js$`,
		},
	}
	for path, expected := range paths {
		t.Run(path, func(t *testing.T) {
			name, loader, parts := pickLoader(path)
			assert.Equal(t, "cdnjs", name)
			assert.Equal(t, expected.parts, parts)
			src, err := loader(path, parts)
			assert.NoError(t, err)
			assert.Regexp(t, expected.src, src)
			data, err := Load(afero.NewMemMapFs(), "/", path)
			if assert.NoError(t, err) {
				assert.Equal(t, path, data.Filename)
				assert.NotEmpty(t, data.Data)
			}
		})
	}

	t.Run("cdnjs.com/libraries/nonexistent", func(t *testing.T) {
		path := "cdnjs.com/libraries/nonexistent"
		name, loader, parts := pickLoader(path)
		assert.Equal(t, "cdnjs", name)
		assert.Equal(t, []string{"nonexistent", "", ""}, parts)
		_, err := loader(path, parts)
		assert.EqualError(t, err, "cdnjs: no such library: nonexistent")
	})

	t.Run("cdnjs.com/libraries/Faker/3.1.0/nonexistent.js", func(t *testing.T) {
		path := "cdnjs.com/libraries/Faker/3.1.0/nonexistent.js"
		name, loader, parts := pickLoader(path)
		assert.Equal(t, "cdnjs", name)
		assert.Equal(t, []string{"Faker", "3.1.0", "nonexistent.js"}, parts)
		src, err := loader(path, parts)
		assert.NoError(t, err)
		assert.Equal(t, "https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/nonexistent.js", src)
		_, err = Load(afero.NewMemMapFs(), "/", path)
		assert.EqualError(t, err, "cdnjs: not found: https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/nonexistent.js")
	})
}
