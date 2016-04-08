package runner

import (
	"errors"
	"github.com/loadimpact/speedboat/runner/js"
	"path"
)

func Get(filename string) (Runner, error) {
	switch path.Ext(filename) {
	case "js":
		return js.New()
	default:
		return nil, errors.New("No runner found")
	}
}
