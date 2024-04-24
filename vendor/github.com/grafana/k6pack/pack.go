// Package k6pack contains a k6 TypeScript/JavaScript script packager.
package k6pack

import (
	"github.com/evanw/esbuild/pkg/api"
	"github.com/grafana/k6pack/internal/plugins/http"
	"github.com/grafana/k6pack/internal/plugins/k6"
)

// Pack transforms TypeScript/JavaScript sources into single k6 compatible JavaScript test script.
func Pack(source string, opts *Options) ([]byte, error) {
	return pack(source, opts.setDefaults())
}

func pack(source string, opts *Options) ([]byte, error) {
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
	})

	if has, err := checkError(&result); has {
		return nil, err
	}

	return result.OutputFiles[0].Contents, nil
}
