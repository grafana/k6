package k6pack

import (
	"path/filepath"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

// Options used to specify transform/build options.
type Options struct {
	Directory  string
	Filename   string
	Timeout    time.Duration
	SourceMap  bool
	Minify     bool
	TypeScript bool
	Externals  []string
	SourceRoot string
}

func (o *Options) setDefaults() *Options {
	if !o.TypeScript {
		o.TypeScript = filepath.Ext(o.Filename) == ".ts"
	}

	return o
}

func (o *Options) loaderType() api.Loader {
	if o.TypeScript {
		return api.LoaderTS
	}

	return api.LoaderJS
}

func (o *Options) sourceMapType() api.SourceMap {
	if o.SourceMap {
		return api.SourceMapInline
	}

	return api.SourceMapNone
}

func (o *Options) stdinOptions(contents string) *api.StdinOptions {
	dir := filepath.Dir(o.Filename)
	base := filepath.Base(o.Filename)
	if base == "." { // empty filename = stdin
		base = ""
	}

	return &api.StdinOptions{
		Contents:   contents,
		Sourcefile: base,
		Loader:     o.loaderType(),
		ResolveDir: dir,
	}
}
