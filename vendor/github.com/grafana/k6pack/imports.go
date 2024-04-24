package k6pack

import (
	"encoding/json"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/grafana/k6pack/internal/plugins/http"
	"github.com/grafana/k6pack/internal/plugins/k6"
)

// Imports returns k6 imports from input script.
func Imports(source string, opts *Options) ([]string, error) {
	return imports(source, opts.setDefaults())
}

func imports(source string, opts *Options) ([]string, error) {
	result := api.Build(api.BuildOptions{ //nolint:exhaustruct
		Stdin:    opts.stdinOptions(source),
		Bundle:   true,
		LogLevel: api.LogLevelSilent,
		Metafile: true,
		Plugins:  []api.Plugin{http.New(), k6.New()},
		External: opts.Externals,
	})

	if has, err := checkError(&result); has {
		return nil, err
	}

	var data metafile

	err := json.Unmarshal([]byte(result.Metafile), &data)
	if err != nil {
		return nil, wrapError(err)
	}

	return data.K6.Imports, nil
}

type metafile struct {
	K6 *k6.Metadata `json:"k6,omitempty"`
}
