package lua

import (
	"github.com/yuin/gopher-lua"
	"time"
)

func (vu *VUContext) Sleep(L *lua.LState) int {
	t := L.ToNumber(1)
	time.Sleep(time.Duration(t) * time.Second)
	return 0
}
