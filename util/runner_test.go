package util

import (
	"github.com/loadimpact/speedboat/runner/js"
	"testing"
)

func GetRunnerJS(t *testing.T) {
	r, err := GetRunner("script.js")
	if err != nil {
		t.Error(err)
	}
	if _, ok := r.(*js.JSRunner); !ok {
		t.Error("Not a JS runner")
	}
}

func GetRunnerUnknown(t *testing.T) {
	r, err := GetRunner("test.doc")
	if err == nil {
		t.Error("No error")
	}
	if r != nil {
		t.Error("Something returned")
	}
}
