// Package pack implements the functionality to analize a script and all its dependencies
// Code adapted from github.com/grafana/k6pack
package pack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"text/scanner"
	"time"

	"github.com/evanw/esbuild/pkg/api"

	"github.com/grafana/k6deps/internal/pack/plugins/file"
	"github.com/grafana/k6deps/internal/pack/plugins/http"
	"github.com/grafana/k6deps/internal/pack/plugins/k6"
	"github.com/grafana/k6deps/internal/rootfs"
)

// Metadata holds k6 related metadata, emitted under "k6" key of Metafile.
type Metadata struct {
	// Imports contains a list of k6 imports (core modules and extensions).
	Imports []string `json:"imports,omitempty"`
}

type packError struct {
	messages []api.Message
}

// Options used to specify transform/build options.
type Options struct {
	FS         rootfs.FS
	Directory  string
	Filename   string
	Timeout    time.Duration
	TypeScript bool
	Externals  []string
	SourceRoot string
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

func (o *Options) loaderType() api.Loader {
	if o.TypeScript || filepath.Ext(o.Filename) == ".ts" {
		return api.LoaderTS
	}

	return api.LoaderJS
}

func (o *Options) fs() (rootfs.FS, error) {
	if o.FS != nil {
		return o.FS, nil
	}

	if len(o.Directory) > 0 {
		return rootfs.NewFromDir(o.Directory)
	}

	wdir, err := os.Getwd() //nolint:forbidigo
	if err != nil {
		return nil, err
	}

	return rootfs.NewFromDir(wdir)
}

// Pack gathers dependencies and transforms TypeScript/JavaScript sources into single k6 compatible JavaScript test
// script.
func Pack(source string, opts *Options) ([]byte, *Metadata, error) {
	fs, err := opts.fs()
	if err != nil {
		return nil, nil, err
	}

	result := api.Build(api.BuildOptions{ //nolint:exhaustruct
		Stdin:      opts.stdinOptions(source),
		Bundle:     true,
		LogLevel:   api.LogLevelSilent,
		Sourcemap:  api.SourceMapNone,
		SourceRoot: opts.SourceRoot,
		Plugins:    []api.Plugin{http.New(), k6.New(), file.New(fs)},
		External:   opts.Externals,
		Metafile:   true,
		// esbuild makes all relative paths in the code absolute by joining this working dir
		// we must pass this from the fs. Otherwise pack uses the CWD
		AbsWorkingDir: fs.Root(),
	})
	if has, err := checkError(&result); has {
		return nil, nil, err
	}

	metadata, err := parseMetadata(result.Metafile)
	if err != nil {
		return nil, nil, err
	}

	return result.OutputFiles[0].Contents, metadata, nil
}

func parseMetadata(metafile string) (*Metadata, error) {
	var k6meta struct {
		K6 *k6.Metadata `json:"k6,omitempty"`
	}

	err := json.Unmarshal([]byte(metafile), &k6meta)
	if err != nil {
		return nil, wrapError(err)
	}

	return &Metadata{
		Imports: k6meta.K6.Imports,
	}, nil
}

func checkError(result *api.BuildResult) (bool, error) {
	if len(result.Errors) == 0 {
		return false, nil
	}

	return true, &packError{messages: result.Errors}
}

func wrapError(err error) error {
	return &packError{[]api.Message{{Text: err.Error()}}}
}

func (e *packError) Error() string {
	msg := e.messages[0]

	if msg.Location == nil {
		return msg.Text
	}

	pos := scanner.Position{
		Filename: msg.Location.File,
		Line:     msg.Location.Line,
		Column:   msg.Location.Column,
	}

	return pos.String() + " " + msg.Text
}
