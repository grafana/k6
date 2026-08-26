package testscript

import (
	"path/filepath"
	"testing"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/js/modulestest"
)

// RunFile runs a single testscript file using a minimal test runtime.
// It uses registered extensions if no module pairs are provided.
func RunFile(t *testing.T, filename string, modulePairs ...any) {
	t.Helper()

	t.Attr("script", filename)

	testRuntime := newTestRuntime(t, modulePairs...)
	state := newTestVUState(t)
	runtime := testRuntime.VU.Runtime()

	module := runtime.NewObject()
	exports := runtime.NewObject()

	set := func(obj *sobek.Object, key string, val any) {
		t.Helper()

		if err := obj.Set(key, val); err != nil {
			t.Fatalf("%s: Set property %q failed: %v", filename, key, err)
		}
	}

	set(module, "exports", exports)
	set(runtime.GlobalObject(), "module", module)
	set(runtime.GlobalObject(), "exports", exports)

	prog, err := modulestest.CompileFile(filepath.Dir(filename), filepath.Base(filename))
	if err != nil {
		t.Fatalf("%s: compile failed: %v", filename, err)
	}

	_, err = runtime.RunProgram(prog)
	if err != nil {
		t.Fatalf("%s: run failed: %v", filename, err)
	}

	testRuntime.MoveToVUContext(state)

	result, err := runtime.RunString("module.exports")
	if err != nil {
		t.Fatalf("%s: failed to get module.exports: %v", filename, err)
	}

	exports = result.ToObject(runtime)

	fn, ok := sobek.AssertFunction(exports.Get("default"))
	if !ok {
		t.Fatalf("%s: exports.default should be a function", filename)
	}

	_, err = fn(sobek.Undefined())
	if err != nil {
		t.Fatalf("%s: default() failed: %v", filename, err)
	}

	testRuntime.EventLoop.WaitOnRegistered()
}

// RunFiles runs all provided testscript files using a minimal test runtime.
// It uses registered extensions if no module pairs are provided.
func RunFiles(t *testing.T, files []string, modulePairs ...any) {
	t.Helper()

	if len(files) == 0 {
		t.Fatal("no test files provided")

		return
	}

	modulePairs = modulePairsOrRegistered(t, modulePairs)

	for _, file := range files {
		t.Run(filepath.ToSlash(file), func(t *testing.T) {
			RunFile(t, file, modulePairs...)
		})
	}
}

// RunGlob runs all testscript files matching the provided glob pattern
// using a minimal test runtime.
// It uses registered extensions if no module pairs are provided.
func RunGlob(t *testing.T, glob string, modulePairs ...any) {
	t.Helper()

	files, err := filepath.Glob(glob)
	if err != nil {
		t.Fatalf("glob %q failed: %v", glob, err)

		return
	}

	RunFiles(t, files, modulePairs...)
}

func registeredExtensions(t *testing.T) []any {
	t.Helper()

	extensions := ext.Get(ext.JSExtension)
	pairs := make([]any, 0, len(extensions)<<1)

	for path, e := range extensions {
		pairs = append(pairs, path, e.Module)
	}

	return pairs
}

func modulePairsOrRegistered(t *testing.T, modulePairs []any) []any {
	t.Helper()

	if len(modulePairs)%2 != 0 {
		t.Fatalf("modulePairs length must be even, got %d", len(modulePairs))
	}

	if len(modulePairs) == 0 {
		modulePairs = registeredExtensions(t)
	}

	return modulePairs
}
