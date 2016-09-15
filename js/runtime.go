package js

import (
	"bytes"
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const wrapper = "(function() { var e = {}; (function(exports) {%s\n})(e); return e; })();"

var libBox = rice.MustFindBox("lib")

type Runtime struct {
	VM      *otto.Otto
	Root    string
	Exports map[string]otto.Value
	Lib     map[string]otto.Value
}

func New() (*Runtime, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return &Runtime{
		VM:      otto.New(),
		Root:    wd,
		Exports: make(map[string]otto.Value),
		Lib:     make(map[string]otto.Value),
	}, nil
}

func (r *Runtime) Load(filename string) (otto.Value, error) {
	// You can only require() modules during setup; doing it during runtime would sink your test's
	// performance in potentially nonobvious ways. Only relative imports ("./*", "../*") are
	// allowed for user code; absolute imports are used for system-provided libraries.
	r.VM.Set("require", func(call otto.FunctionCall) otto.Value {
		name := call.Argument(0).String()
		if !strings.HasPrefix(name, ".") {
			exports, err := r.loadLib(name + ".js")
			if err != nil {
				panic(call.Otto.MakeCustomError("ImportError", err.Error()))
			}
			return exports
		}

		exports, err := r.loadFile(name + ".js")
		if err != nil {
			panic(call.Otto.MakeCustomError("ImportError", err.Error()))
		}
		return exports
	})
	defer r.VM.Set("require", nil)

	return r.loadFile(filename)
}

func (r *Runtime) ExtractOptions(exports otto.Value, opts *lib.Options) error {
	expObj := exports.Object()
	if expObj == nil {
		return nil
	}

	v, err := expObj.Get("options")
	if err != nil {
		return err
	}
	obj := v.Object()
	if obj == nil {
		return nil
	}

	for _, key := range obj.Keys() {
		val, err := obj.Get(key)
		if err != nil {
			return err
		}

		switch key {
		case "vus":
			vus, err := val.ToInteger()
			if err != nil {
				return err
			}
			opts.VUs = vus
		case "vusMax":
			vusMax, err := val.ToInteger()
			if err != nil {
				return err
			}
			opts.VUsMax = vusMax
		case "duration":
			seconds, err := val.ToFloat()
			if err != nil {
				return err
			}
			opts.Duration = time.Duration(seconds * float64(time.Second))
		}
	}

	return nil
}

func (r *Runtime) loadFile(filename string) (otto.Value, error) {
	// To protect against directory traversal, prevent loading of files outside the root (pwd) dir
	path, err := filepath.Abs(filename)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	if !strings.HasPrefix(path, r.Root) {
		return otto.UndefinedValue(), DirectoryTraversalError{Filename: filename, Root: r.Root}
	}

	// Don't re-compile repeated includes of the same module
	if exports, ok := r.Exports[path]; ok {
		return exports, nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	exports, err := r.load(path, data)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	r.Exports[path] = exports

	log.WithField("path", path).Debug("File loaded")

	return exports, nil
}

func (r *Runtime) loadLib(filename string) (otto.Value, error) {
	if exports, ok := r.Lib[filename]; ok {
		return exports, nil
	}

	data, err := libBox.Bytes(filename)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	exports, err := r.load(filename, data)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	r.Lib[filename] = exports

	log.WithField("filename", filename).Debug("Library loaded")

	return exports, nil
}

func (r *Runtime) load(filename string, data []byte) (otto.Value, error) {
	// Compile the file with Babel; this subprocess invocation is TEMPORARY:
	// https://github.com/robertkrimen/otto/pull/205
	cmd := exec.Command(babel, "--presets", "latest", "--no-babelrc")
	cmd.Dir = babelDir
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = os.Stderr
	src, err := cmd.Output()
	if err != nil {
		return otto.UndefinedValue(), err
	}

	// Use a wrapper function to turn the script into an exported module
	s, err := r.VM.Compile(filename, fmt.Sprintf(wrapper, string(src)))
	if err != nil {
		return otto.UndefinedValue(), err
	}
	exports, err := r.VM.Run(s)
	if err != nil {
		return otto.UndefinedValue(), err
	}

	return exports, nil
}
