package js

import (
	"github.com/kardianos/osext"
	"path"
)

var (
	babelDir = "."
	babel    = "babel"
)

func init() {
	if dir, err := osext.ExecutableFolder(); err == nil {
		babelDir = path.Join(dir, "js")
		babel = path.Join(babelDir, "node_modules", ".bin", babel)
	}
}
