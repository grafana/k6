/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

const wrapper = "(function() { var e = {}; (function(exports) {%s\n})(e); return e; })();"

var (
	libBox      = rice.MustFindBox("lib")
	polyfillBox = rice.MustFindBox("node_modules/babel-polyfill")
)

type Runtime struct {
	VM      *otto.Otto
	Exports map[string]otto.Value
	Metrics map[string]*stats.Metric
	Options lib.Options

	lib map[string]otto.Value
}

func New() (*Runtime, error) {
	rt := &Runtime{
		VM:      otto.New(),
		Exports: make(map[string]otto.Value),
		Metrics: make(map[string]*stats.Metric),
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

	__ENV := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v := lib.SplitKV(kv)
		__ENV[k] = v
	}
	if err := rt.VM.Set("__ENV", __ENV); err != nil {
		return nil, err
	}

	return rt, nil
}

func (r *Runtime) Load(src *lib.SourceData) (otto.Value, error) {
	if err := r.VM.Set("__initapi__", &InitAPI{r: r}); err != nil {
		return otto.UndefinedValue(), err
	}
	exp, err := r.loadSource(src)
	if err := r.VM.Set("__initapi__", nil); err != nil {
		return otto.UndefinedValue(), err
	}
	return exp, err
}

func (r *Runtime) extractOptions(exports otto.Value, opts *lib.Options) error {
	expObj := exports.Object()
	if expObj == nil {
		return nil
	}

	v, err := expObj.Get("options")
	if err != nil {
		return err
	}
	ev, err := v.Export()
	if err != nil {
		return err
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, opts); err != nil {
		return err
	}

	return nil
}

func (r *Runtime) loadSource(src *lib.SourceData) (otto.Value, error) {
	path, err := filepath.Abs(src.Filename)
	if err != nil {
		return otto.UndefinedValue(), err
	}

	// Don't re-compile repeated includes of the same module
	if exports, ok := r.Exports[path]; ok {
		return exports, nil
	}
	exports, err := r.load(path, src.Data)
	if err != nil {
		return otto.UndefinedValue(), err
	}
	r.Exports[path] = exports

	log.WithField("path", path).Debug("File loaded")

	return exports, nil
}

func (r *Runtime) loadFile(filename string) (otto.Value, error) {
	path, err := filepath.Abs(filename)
	if err != nil {
		return otto.UndefinedValue(), err
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
	nodeNames := []string{"node", "nodejs", "node.exe"}
	var nodePath string
	for _, name := range nodeNames {
		path, err := exec.LookPath(name)
		if err != nil {
			if e, ok := err.(*exec.Error); ok && e.Err != exec.ErrNotFound {
				return otto.UndefinedValue(), err
			}
			continue
		}
		nodePath = path
		break
	}
	if nodePath == "" {
		return otto.UndefinedValue(), errors.New(
			"Couldn't find node, make sure it's in your PATH. " +
				"This is a TEMPORARY dependency and will be removed. " +
				"See: https://github.com/loadimpact/k6/issues/14",
		)
	}

	// Compile the file with Babel; this subprocess invocation is TEMPORARY:
	// https://github.com/robertkrimen/otto/pull/205
	cmd := exec.Command(nodePath, babel, "--presets", "latest", "--no-babelrc")
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

	// Extract script-defined options.
	var opts lib.Options
	if err := r.extractOptions(exports, &opts); err != nil {
		return exports, err
	}
	r.Options = r.Options.Apply(opts)

	return exports, nil
}
