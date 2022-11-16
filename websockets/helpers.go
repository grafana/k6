package websockets

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

// must is a small helper that will panic if err is not nil.
func must(rt *goja.Runtime, err error) {
	if err != nil {
		common.Throw(rt, err)
	}
}
