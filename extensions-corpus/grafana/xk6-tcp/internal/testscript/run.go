package testscript

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

// RunFile runs a CommonJS test script using the standalone extension API host.
func RunFile(t *testing.T, filename string, modulePairs ...any) {
	t.Helper()
	t.Attr("script", filename)

	runtime := extensionapitest.NewScriptRuntime(nil)
	runtime.SetModule("k6/execution", map[string]any{"test": map[string]any{
		"fail":  func(message string) { t.Error(message) },
		"abort": func(message string) { t.Fatal(message) },
	}})

	for i := 0; i+1 < len(modulePairs); i += 2 {
		path, ok := modulePairs[i].(string)
		if !ok {
			t.Fatalf("module pair at index %d: expected string, got %T", i, modulePairs[i])
		}
		module, ok := modulePairs[i+1].(extensionapi.Module)
		if !ok {
			t.Fatalf("module pair %q: expected extension API module, got %T", path, modulePairs[i+1])
		}
		runtime.SetExtension(path, module)
	}

	env := make(map[string]string)
	for _, value := range os.Environ() {
		if key, value, ok := splitEnv(value); ok {
			env[key] = value
		}
	}
	runtime.VU.LookupEnvFunc = func(key string) (string, bool) { value, ok := env[key]; return value, ok }
	if err := runtime.VU.Runtime().Set("__ENV", env); err != nil {
		t.Fatalf("%s: set __ENV failed: %v", filename, err)
	}
	runtime.VU.DialContextFunc = dialContext
	runtime.VU.LookupHostFunc = lookupHost
	runtime.VU.TLSClientFunc = tlsClient
	runtime.VU.CheckHostFunc = func(_ context.Context, _ string) error { return nil }

	exports, err := runtime.RunFile(filename)
	if err != nil {
		t.Fatalf("%s: run failed: %v", filename, err)
	}
	fn, ok := sobek.AssertFunction(exports.ToObject(runtime.VU.Runtime()).Get("default"))
	if !ok {
		t.Fatalf("%s: exports.default should be a function", filename)
	}
	if _, err = runtime.Call(fn); err != nil {
		t.Fatalf("%s: default() failed: %v", filename, err)
	}
}

func RunFiles(t *testing.T, files []string, modulePairs ...any) {
	t.Helper()
	if len(files) == 0 {
		t.Fatal("no test files provided")
		return
	}
	for _, file := range files {
		t.Run(filepath.ToSlash(file), func(t *testing.T) { RunFile(t, file, modulePairs...) })
	}
}

func RunGlob(t *testing.T, glob string, modulePairs ...any) {
	t.Helper()
	files, err := filepath.Glob(glob)
	if err != nil {
		t.Fatalf("glob %q failed: %v", glob, err)
		return
	}
	RunFiles(t, files, modulePairs...)
}

func splitEnv(value string) (string, string, bool) {
	for i := 0; i < len(value); i++ {
		if value[i] == '=' {
			return value[:i], value[i+1:], true
		}
	}
	return "", "", false
}
