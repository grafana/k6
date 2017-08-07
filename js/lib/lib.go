package lib

import (
	"github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
)

var CoreJS *goja.Program = goja.MustCompile(
	"core-js/shim.js",
	rice.MustFindBox("core-js").MustString("client/shim.js"),
	true,
)
