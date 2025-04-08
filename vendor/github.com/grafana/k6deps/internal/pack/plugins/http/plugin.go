// Package http contains esbuild http plugin.
package http

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
)

const (
	pluginName     = "http"
	defaultTimeout = 30 * time.Second
)

var (
	errNotFound     = errors.New("module not found")
	errHTTPForbiden = errors.New("http is forbidden except for 127.0.0.1")
)

var (
	loaderByContentType = map[string]api.Loader{ //nolint:gochecknoglobals
		"text/javascript":        api.LoaderJS,
		"application/json":       api.LoaderJSON,
		"application/typescript": api.LoaderTS,
		"text/typescript":        api.LoaderTS,
		"video/mp2t":             api.LoaderTS,
	}

	loaderByExtension = map[string]api.Loader{ //nolint:gochecknoglobals
		".js":   api.LoaderJS,
		".json": api.LoaderJSON,
		".txt":  api.LoaderText,
		".ts":   api.LoaderTS,
	}
)

type plugin struct {
	client  *http.Client
	timeout time.Duration
	resolve func(path string, options api.ResolveOptions) api.ResolveResult
	options *api.BuildOptions
}

func toURL(namespace, path string) string {
	prefix := namespace + ":"
	if strings.HasPrefix(path, prefix) {
		return path
	}

	return prefix + path
}

// New creates new http plugin instance.
func New() api.Plugin {
	plugin := &plugin{
		client:  &http.Client{},
		timeout: defaultTimeout,
	}

	return api.Plugin{
		Name:  pluginName,
		Setup: plugin.setup,
	}
}

func (plugin *plugin) setup(build api.PluginBuild) {
	plugin.resolve = build.Resolve
	plugin.options = build.InitialOptions

	build.OnResolve(api.OnResolveOptions{Filter: "^https?://"}, plugin.onResolveAbsolute)
	build.OnResolve(api.OnResolveOptions{Filter: ".*", Namespace: "https"}, plugin.onResolve)
	build.OnResolve(api.OnResolveOptions{Filter: ".*", Namespace: "http"}, plugin.onResolve)

	build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "https"}, plugin.onLoad)
	build.OnLoad(api.OnLoadOptions{Filter: ".*", Namespace: "http"}, plugin.onLoad)
}

func (plugin *plugin) load(loc *url.URL) (*api.OnLoadResult, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), plugin.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loc.String(), nil)
	if err != nil {
		return nil, err
	}

	res, err := plugin.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode != http.StatusOK {
		return nil, errNotFound
	}

	contentType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))

	if contentType == "text/html" {
		return nil, errNotFound
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	contents := string(bytes)

	var loader api.Loader

	if ldr, ok := loaderByContentType[contentType]; ok {
		loader = ldr
	} else if ldr, ok := loaderByExtension[path.Ext(loc.Path)]; ok {
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
	if res, ok := args.PluginData.(*api.OnLoadResult); ok {
		return *res, nil
	}

	return onLoadError(args, errNotFound)
}

func onResolveError(_ api.OnResolveArgs, err error) (api.OnResolveResult, error) {
	return api.OnResolveResult{}, err
}

func onLoadError(_ api.OnLoadArgs, err error) (api.OnLoadResult, error) {
	return api.OnLoadResult{}, err
}

func newOnResolveResult(loc *url.URL, plugindata interface{}) api.OnResolveResult {
	return api.OnResolveResult{
		Namespace:  loc.Scheme,
		Path:       "//" + loc.Host + loc.Path,
		PluginData: plugindata,
	}
}

func (plugin *plugin) resolveURL(loc *url.URL, args api.OnResolveArgs) (api.OnResolveResult, error) {
	loadResult, err := plugin.load(loc)
	if err == nil {
		return newOnResolveResult(loc, loadResult), nil
	}

	if !errors.Is(err, errNotFound) || len(path.Ext(loc.Path)) != 0 {
		return onResolveError(args, err)
	}

	orig := loc.Path

	var res api.ResolveResult

	for _, suffix := range []string{".ts", ".js", "/index.ts", "/index.js"} {
		loc.Path = orig + suffix

		res = plugin.resolve(loc.String(), api.ResolveOptions{
			Kind: args.Kind,
		})

		if len(res.Errors) == 0 {
			break
		}
	}

	if len(res.Errors) != 0 {
		return api.OnResolveResult{Errors: res.Errors}, nil
	}

	return newOnResolveResult(loc, res.PluginData), nil
}

func (plugin *plugin) onResolveAbsolute(args api.OnResolveArgs) (api.OnResolveResult, error) {
	for _, ext := range plugin.options.External {
		if strings.HasPrefix(args.Path, ext) {
			return api.OnResolveResult{External: true}, nil
		}
	}

	loc, err := url.Parse(args.Path)
	if err != nil {
		return onResolveError(args, err)
	}

	if loc.Scheme == "http" && loc.Hostname() != "127.0.0.1" {
		return onResolveError(args, errHTTPForbiden)
	}

	return plugin.resolveURL(loc, args)
}

func (plugin *plugin) onResolveRelative(args api.OnResolveArgs) (api.OnResolveResult, error) {
	base, err := url.Parse(toURL(args.Namespace, args.Importer))
	if err != nil {
		return onResolveError(args, err)
	}

	loc, err := url.Parse(args.Path)
	if err != nil {
		return onResolveError(args, err)
	}

	loc = base.ResolveReference(loc)

	return plugin.resolveURL(loc, args)
}

func (plugin *plugin) onResolve(args api.OnResolveArgs) (api.OnResolveResult, error) {
	if args.Path[0] == '.' {
		return plugin.onResolveRelative(args)
	}

	if strings.HasPrefix(args.Path, args.Namespace+":") {
		return plugin.onResolveAbsolute(args)
	}

	res := plugin.resolve(args.Path, api.ResolveOptions{Importer: args.Importer, Kind: args.Kind})

	return api.OnResolveResult{
			Errors:     res.Errors,
			Path:       res.Path,
			External:   res.External,
			Namespace:  res.Namespace,
			Suffix:     res.Suffix,
			PluginData: res.PluginData,
		},
		nil
}
