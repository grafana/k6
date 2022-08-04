package loader

import (
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
)

func TestCDNJS(t *testing.T) {
	t.Skip("skipped to avoid inconsistent API responses")
	t.Parallel()

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

	root := &url.URL{Scheme: "https", Host: "example.com", Path: "/something/"}
	logger := logrus.New()
	logger.SetOutput(testutils.NewTestOutput(t))
	for path, expected := range paths {
		path, expected := path, expected
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			name, loader, parts := pickLoader(path)
			assert.Equal(t, "cdnjs", name)
			assert.Equal(t, expected.parts, parts)

			src, err := loader(logger, path, parts)
			require.NoError(t, err)
			assert.Regexp(t, expected.src, src)

			resolvedURL, err := Resolve(root, path)
			require.NoError(t, err)
			require.Empty(t, resolvedURL.Scheme)
			require.Equal(t, path, resolvedURL.Opaque)

			data, err := Load(logger, map[string]afero.Fs{"https": afero.NewMemMapFs()}, resolvedURL, path)
			require.NoError(t, err)
			assert.Equal(t, resolvedURL, data.URL)
			assert.NotEmpty(t, data.Data)
		})
	}

	t.Run("cdnjs.com/libraries/nonexistent", func(t *testing.T) {
		t.Parallel()
		path := "cdnjs.com/libraries/nonexistent"
		name, loader, parts := pickLoader(path)
		assert.Equal(t, "cdnjs", name)
		assert.Equal(t, []string{"nonexistent", "", ""}, parts)
		_, err := loader(logger, path, parts)
		assert.EqualError(t, err, "cdnjs: no such library: nonexistent")
	})

	t.Run("cdnjs.com/libraries/Faker/3.1.0/nonexistent.js", func(t *testing.T) {
		t.Parallel()
		path := "cdnjs.com/libraries/Faker/3.1.0/nonexistent.js"
		name, loader, parts := pickLoader(path)
		assert.Equal(t, "cdnjs", name)
		assert.Equal(t, []string{"Faker", "3.1.0", "nonexistent.js"}, parts)
		src, err := loader(logger, path, parts)
		require.NoError(t, err)
		assert.Equal(t, "https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/nonexistent.js", src)

		pathURL, err := url.Parse(src)
		require.NoError(t, err)

		_, err = Load(logger, map[string]afero.Fs{"https": afero.NewMemMapFs()}, pathURL, path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found: https://cdnjs.cloudflare.com/ajax/libs/Faker/3.1.0/nonexistent.js")
	})
}
