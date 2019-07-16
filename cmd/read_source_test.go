package cmd

import (
	"bytes"
	"io"
	"net/url"
	"testing"

	"github.com/loadimpact/k6/lib/fsext"
	"github.com/loadimpact/k6/loader"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type errorReader string

func (e errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New((string)(e))
}

var _ io.Reader = errorReader("")

func TestReadSourceSTDINError(t *testing.T) {
	_, err := readSource("-", "", nil, errorReader("1234"))
	require.Error(t, err)
	require.Equal(t, "1234", err.Error())
}

func TestReadSourceSTDINCache(t *testing.T) {
	var data = []byte(`test contents`)
	var r = bytes.NewReader(data)
	var fs = afero.NewMemMapFs()
	sourceData, err := readSource("-", "/path/to/pwd",
		map[string]afero.Fs{"file": fsext.NewCacheOnReadFs(nil, fs, 0)}, r)
	require.NoError(t, err)
	require.Equal(t, &loader.SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/-"},
		Data: data}, sourceData)
	fileData, err := afero.ReadFile(fs, "/-")
	require.NoError(t, err)
	require.Equal(t, data, fileData)
}

func TestReadSourceRelative(t *testing.T) {
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/path/to/somewhere/script.js", data, 0644))
	sourceData, err := readSource("../somewhere/script.js", "/path/to/pwd", map[string]afero.Fs{"file": fs}, nil)
	require.NoError(t, err)
	require.Equal(t, &loader.SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/path/to/somewhere/script.js"},
		Data: data}, sourceData)
}

func TestReadSourceAbsolute(t *testing.T) {
	var data = []byte(`test contents`)
	var r = bytes.NewReader(data)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/a/b", data, 0644))
	require.NoError(t, afero.WriteFile(fs, "/c/a/b", []byte("wrong"), 0644))
	sourceData, err := readSource("/a/b", "/c", map[string]afero.Fs{"file": fs}, r)
	require.NoError(t, err)
	require.Equal(t, &loader.SourceData{
		URL:  &url.URL{Scheme: "file", Path: "/a/b"},
		Data: data}, sourceData)
}

func TestReadSourceHttps(t *testing.T) {
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/github.com/something", data, 0644))
	sourceData, err := readSource("https://github.com/something", "/c",
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": fs}, nil)
	require.NoError(t, err)
	require.Equal(t, &loader.SourceData{
		URL:  &url.URL{Scheme: "https", Host: "github.com", Path: "/something"},
		Data: data}, sourceData)
}

func TestReadSourceHttpError(t *testing.T) {
	var data = []byte(`test contents`)
	var fs = afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/github.com/something", data, 0644))
	_, err := readSource("http://github.com/something", "/c",
		map[string]afero.Fs{"file": afero.NewMemMapFs(), "https": fs}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), `only supported schemes for imports are file and https`)
}
