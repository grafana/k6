package common

import (
	"errors"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"path"
)

func GetRunner(filename string) (runner.Runner, error) {
	switch path.Ext(filename) {
	case ".js":
		return js.New()
	default:
		return nil, errors.New("No runner found")
	}
}
