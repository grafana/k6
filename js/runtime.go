package js

import (
	"bytes"
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v3"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const wrapper = "(function() { var e = {}; (function(exports) {%s\n})(e); return e; })();"

var (
	libBox      = rice.MustFindBox("lib")
	polyfillBox = rice.MustFindBox("node_modules/babel-polyfill")
)

type Runtime struct {
	VM      *otto.Otto
	Root    string
	Exports map[string]otto.Value

	lib map[string]otto.Value
}

func New() (*Runtime, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	rt := &Runtime{
		VM:      otto.New(),
		Root:    wd,
		Exports: make(map[string]otto.Value),
		lib:     make(map[string]otto.Value),
	}

	polyfillJS, err := polyfillBox.String("dist/polyfill.js")
	if err != nil {
		return nil, err
	}
	polyfill, err := rt.VM.Compile("polyfill.js", polyfillJS)
	if err != nil {
		return nil, err
	}
	if _, err := rt.VM.Run(polyfill); err != nil {
		return nil, err
	}

	if _, err := rt.loadLib("_global.js"); err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *Runtime) Load(filename string) (otto.Value, error) {
	r.VM.Set("__initapi__", InitAPI{r: r})
	defer r.VM.Set("__initapi__", nil)

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
			opts.VUs = null.IntFrom(vus)
		case "vusMax":
			vusMax, err := val.ToInteger()
			if err != nil {
				return err
			}
			opts.VUsMax = null.IntFrom(vusMax)
		case "duration":
			duration, err := val.ToString()
			if err != nil {
				return err
			}
			opts.Duration = null.StringFrom(duration)
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
	if exports, ok := r.lib[filename]; ok {
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
	r.lib[filename] = exports

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
