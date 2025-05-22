// Package file contains esbuild file plugin.
package file

import (
	"io"
	"path/filepath"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/grafana/k6deps/internal/rootfs"
)

const (
	pluginName = "file"
)

var loaderByExtension = map[string]api.Loader{ //nolint:gochecknoglobals
	".js":   api.LoaderJS,
	".json": api.LoaderJSON,
	".txt":  api.LoaderText,
	".ts":   api.LoaderTS,
}

type plugin struct {
	fs      rootfs.FS
	resolve func(path string, options api.ResolveOptions) api.ResolveResult
	options *api.BuildOptions
}

// New creates new http plugin instance.
func New(fs rootfs.FS) api.Plugin {
	plugin := &plugin{
		fs: fs,
	}

	return api.Plugin{
		Name:  pluginName,
		Setup: plugin.setup,
	}
}

func (plugin *plugin) setup(build api.PluginBuild) {
	plugin.resolve = build.Resolve
	plugin.options = build.InitialOptions

	build.OnResolve(api.OnResolveOptions{Filter: ".*", Namespace: "file"}, plugin.onResolve)
	build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "file"}, plugin.onLoad)
}

func (plugin *plugin) load(path string) (*api.OnLoadResult, error) {
	file, err := plugin.fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	contents := string(bytes)

	// FIXME: what happens if the extension is not supported?
	var loader api.Loader
	if ldr, ok := loaderByExtension[filepath.Ext(path)]; ok {
		loader = ldr
	}

	return &api.OnLoadResult{
			Contents:   &contents,
			PluginName: pluginName,
			Loader:     loader,
		},
		nil
}

func (plugin *plugin) onLoad(args api.OnLoadArgs) (api.OnLoadResult, error) {
	loadResult, err := plugin.load(args.Path)
	if err != nil {
		return onLoadError(args, err)
	}

	return *loadResult, nil
}

func onLoadError(_ api.OnLoadArgs, err error) (api.OnLoadResult, error) {
	return api.OnLoadResult{}, err
}

func newOnResolveResult(path string) api.OnResolveResult {
	return api.OnResolveResult{
		Namespace: "file",
		Path:      path,
	}
}

func (plugin *plugin) onResolve(args api.OnResolveArgs) (api.OnResolveResult, error) {
	path := args.Path

	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(args.Importer), path)
	}

	return newOnResolveResult(path), nil
}
