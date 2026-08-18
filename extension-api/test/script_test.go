package extensionapitest

import (
	"testing"

	"github.com/grafana/sobek"
)

func TestScriptRuntime(t *testing.T) {
	t.Parallel()

	runtime := NewScriptRuntime(map[string]any{"k6/x/example": map[string]any{"value": 42}})
	exports, err := runtime.RunSource("example.cjs", `
		const example = require("k6/x/example");
		module.exports = function () {
			if (example.value !== 42) { throw new Error("unexpected module export"); }
			return "ok";
		};
	`)
	if err != nil {
		t.Fatal(err)
	}
	function, ok := sobek.AssertFunction(exports)
	if !ok {
		t.Fatal("module.exports is not callable")
	}
	result, err := runtime.Call(function)
	if err != nil {
		t.Fatal(err)
	}
	if actual := result.String(); actual != "ok" {
		t.Fatalf("unexpected result %q", actual)
	}
}

func TestScriptRuntimeUnknownModule(t *testing.T) {
	t.Parallel()

	runtime := NewScriptRuntime(nil)
	_, err := runtime.RunSource("unknown.cjs", `require("k6/x/missing")`)
	if err == nil {
		t.Fatal("expected unknown module error")
	}
}
