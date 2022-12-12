package ext

import (
	"fmt"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
)

// TODO: Make an ExtensionRegistry?
//
//nolint:gochecknoglobals
var (
	mx         sync.RWMutex
	extensions = make(map[ExtensionType]map[string]*Extension)
)

// ExtensionType is the type of all supported k6 extensions.
type ExtensionType uint8

// All supported k6 extension types.
const (
	JSExtension ExtensionType = iota + 1
	OutputExtension
)

func (e ExtensionType) String() string {
	var s string
	switch e {
	case JSExtension:
		s = "js"
	case OutputExtension:
		s = "output"
	}
	return s
}

// Extension is a generic container for any k6 extension.
type Extension struct {
	Name, Path, Version string
	Type                ExtensionType
	Module              interface{}
}

func (e Extension) String() string {
	return fmt.Sprintf("%s %s, %s [%s]", e.Path, e.Version, e.Name, e.Type)
}

// Register a new extension with the given name and type. This function will
// panic if an unsupported extension type is provided, or if an extension of the
// same type and name is already registered.
func Register(name string, typ ExtensionType, mod interface{}) {
	mx.Lock()
	defer mx.Unlock()

	exts, ok := extensions[typ]
	if !ok {
		panic(fmt.Sprintf("unsupported extension type: %T", typ))
	}

	if _, ok := exts[name]; ok {
		panic(fmt.Sprintf("extension already registered: %s", name))
	}

	path, version := extractModuleInfo(mod)

	exts[name] = &Extension{
		Name:    name,
		Type:    typ,
		Module:  mod,
		Path:    path,
		Version: version,
	}
}

// Get returns all extensions of the specified type.
func Get(typ ExtensionType) map[string]*Extension {
	mx.RLock()
	defer mx.RUnlock()

	exts, ok := extensions[typ]
	if !ok {
		panic(fmt.Sprintf("unsupported extension type: %T", typ))
	}

	result := make(map[string]*Extension, len(exts))

	for name, ext := range exts {
		result[name] = ext
	}

	return result
}

// GetAll returns all extensions, sorted by their import path and name.
func GetAll() []*Extension {
	mx.RLock()
	defer mx.RUnlock()

	js, out := extensions[JSExtension], extensions[OutputExtension]
	result := make([]*Extension, 0, len(js)+len(out))

	for _, e := range js {
		result = append(result, e)
	}
	for _, e := range out {
		result = append(result, e)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Name < result[j].Name
		}
		return result[i].Path < result[j].Path
	})

	return result
}

// extractModuleInfo attempts to return the package path and version of the Go
// module that created the given value.
func extractModuleInfo(mod interface{}) (path, version string) {
	t := reflect.TypeOf(mod)

	switch t.Kind() {
	case reflect.Ptr:
		if t.Elem() != nil {
			path = t.Elem().PkgPath()
		}
	case reflect.Func:
		path = runtime.FuncForPC(reflect.ValueOf(mod).Pointer()).Name()
	default:
		return
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, dep := range buildInfo.Deps {
		depPath := strings.TrimSpace(dep.Path)
		if strings.HasPrefix(path, depPath) {
			if dep.Replace != nil {
				return depPath, dep.Replace.Version
			}
			return depPath, dep.Version
		}
	}

	return
}

func init() {
	extensions[JSExtension] = make(map[string]*Extension)
	extensions[OutputExtension] = make(map[string]*Extension)
}
