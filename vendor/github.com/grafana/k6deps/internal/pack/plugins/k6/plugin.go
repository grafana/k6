// Package k6 contains esbuild k6 plugin.
package k6

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
)

const (
	pluginName = "k6"
	builtin    = "k6"

	filterSuffix    = "(\\?[^#]*)?(#.*)?$"
	filterBuiltin   = "^k6(/.*)?" + filterSuffix
	filterExtension = "^(@[a-zA-Z0-9-_]+/)?xk6-([a-zA-Z0-9-_]+)((/[a-zA-Z0-9-_]+)*)" + filterSuffix
)

var reExtension = regexp.MustCompile(filterExtension)

// Metadata holds k6 related metadata, emitted under "k6" key of Metafile.
type Metadata struct {
	// Imports contains a list of k6 imports (core modules and extensions).
	Imports []string `json:"imports,omitempty"`
}

type plugin struct {
	options *api.BuildOptions
	resolve func(path string, options api.ResolveOptions) api.ResolveResult
	mu      sync.RWMutex
	imports map[string]struct{}
}

// New creates new k6 plugin instance.
func New() api.Plugin {
	plugin := new(plugin)
	plugin.imports = make(map[string]struct{})

	return api.Plugin{
		Name:  pluginName,
		Setup: plugin.setup,
	}
}

func setOptions(opts *api.BuildOptions) {
	if opts.Define == nil {
		opts.Define = make(map[string]string)
	}

	opts.Define["global"] = "globalThis"

	if opts.Platform == api.PlatformDefault {
		opts.Platform = api.PlatformNeutral
	}

	if opts.Target == api.DefaultTarget {
		opts.Target = api.ESNext
	}

	if opts.Format == api.FormatDefault {
		opts.Format = api.FormatESModule
	}

	if opts.LegalComments == api.LegalCommentsDefault {
		opts.LegalComments = api.LegalCommentsNone
	}
}

func (plugin *plugin) setup(build api.PluginBuild) {
	plugin.resolve = build.Resolve
	plugin.options = build.InitialOptions
	setOptions(plugin.options)

	build.OnResolve(api.OnResolveOptions{Filter: filterBuiltin}, plugin.onResolve)
	build.OnResolve(api.OnResolveOptions{Filter: filterExtension}, plugin.onResolveExtension)
	build.OnEnd(plugin.onEnd)
}

func trimImportPath(ipath string) string {
	loc, err := url.Parse(ipath)
	if err != nil {
		return ipath
	}

	loc.Fragment = ""
	loc.RawQuery = ""

	return loc.String()
}

func (plugin *plugin) addImport(value string) {
	if !plugin.options.Metafile {
		return
	}

	plugin.mu.Lock()
	plugin.imports[value] = struct{}{}
	plugin.mu.Unlock()
}

func (plugin *plugin) onResolveExtension(args api.OnResolveArgs) (api.OnResolveResult, error) {
	plugin.addImport(args.Path)

	subs := reExtension.FindSubmatch([]byte(args.Path))
	rewrite := "k6/x/" + string(subs[2]) + string(subs[3]) + string(subs[5])

	res := plugin.resolve(rewrite, api.ResolveOptions{Kind: args.Kind})

	return api.OnResolveResult{Errors: res.Errors, External: res.External, Path: res.Path}, nil
}

func (plugin *plugin) onResolve(args api.OnResolveArgs) (api.OnResolveResult, error) {
	plugin.addImport(args.Path)

	return api.OnResolveResult{External: true, Path: trimImportPath(args.Path)}, nil
}

func addMetadata(orig string, meta *Metadata) (string, error) {
	var dict map[string]interface{}

	err := json.Unmarshal([]byte(orig), &dict)
	if err != nil {
		return "", err
	}

	dict["k6"] = meta

	var buff bytes.Buffer

	encoder := json.NewEncoder(&buff)
	encoder.SetIndent("", "  ")

	err = encoder.Encode(dict)
	if err != nil {
		return "", err
	}

	return buff.String(), nil
}

func (plugin *plugin) onEnd(result *api.BuildResult) (api.OnEndResult, error) {
	// Skip if metadata was not requested or the esbuild result is missing it due to an error
	if !plugin.options.Metafile || len(result.Metafile) == 0 {
		return api.OnEndResult{}, nil
	}

	imports := make([]string, 0, len(plugin.imports))

	for key := range plugin.imports {
		imports = append(imports, key)
	}

	sort.Strings(imports)

	metafile, err := addMetadata(result.Metafile, &Metadata{Imports: imports})
	if err != nil {
		return api.OnEndResult{}, err
	}

	result.Metafile = metafile

	return api.OnEndResult{}, nil
}
