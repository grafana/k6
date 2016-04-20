package v8js

import (
	"strconv"
	"time"
)

func (vu *VUContext) Sleep(ts string) {
	t, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Duration(t) * time.Second)
}
