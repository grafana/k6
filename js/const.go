package js

import (
	"github.com/kardianos/osext"
	"os"
	"path"
)

var (
	babelDir = "."
	babel    = "babel"
)

func init() {
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		babelDir = path.Join(gopath, "src", "github.com", "loadimpact", "speedboat", "js")
	} else if dir, err := osext.ExecutableFolder(); err == nil {
		babelDir = path.Join(dir, "js")
	}
	babel = path.Join(babelDir, "node_modules", ".bin", babel)
}
