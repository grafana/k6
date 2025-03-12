// Package k6pack contains a k6 TypeScript/JavaScript script packager.
package k6pack

import (
	"encoding/json"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/grafana/k6pack/internal/plugins/http"
	"github.com/grafana/k6pack/internal/plugins/k6"
)

// Metadata holds k6 related metadata, emitted under "k6" key of Metafile.
type Metadata struct {
	// Imports contains a list of k6 imports (core modules and extensions).
	Imports []string `json:"imports,omitempty"`
}

// Pack transforms TypeScript/JavaScript sources into single k6 compatible JavaScript test script.
func Pack(source string, opts *Options) ([]byte, *Metadata, error) {
	return pack(source, opts.setDefaults())
}

func pack(source string, opts *Options) ([]byte, *Metadata, error) {
	result := api.Build(api.BuildOptions{ //nolint:exhaustruct
		Stdin:             opts.stdinOptions(source),
		Bundle:            true,
		MinifyWhitespace:  opts.Minify,
		MinifyIdentifiers: opts.Minify,
		MinifySyntax:      opts.Minify,
		LogLevel:          api.LogLevelSilent,
		Sourcemap:         opts.sourceMapType(),
		SourceRoot:        opts.SourceRoot,
		Plugins:           []api.Plugin{http.New(), k6.New()},
		External:          opts.Externals,
		Metafile:          true,
	})

	if has, err := checkError(&result); has {
		return nil, nil, err
	}

	var meta metafile

	err := json.Unmarshal([]byte(result.Metafile), &meta)
	if err != nil {
		return nil, nil, wrapError(err)
	}

	metadata := &Metadata{
		Imports: meta.K6.Imports,
	}

	return result.OutputFiles[0].Contents, metadata, nil
}

type metafile struct {
	K6 *k6.Metadata `json:"k6,omitempty"`
}
