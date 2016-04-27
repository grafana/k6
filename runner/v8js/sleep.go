package v8js

import (
	"time"
)

func (vu *VUContext) Sleep(t float64) {
	time.Sleep(time.Duration(t) * time.Second)
}
