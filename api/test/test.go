package test

import (
	"github.com/loadimpact/speedboat/runner"
)

var Members = map[string]interface{}{
	"abort": Abort,
}

func Abort() <-chan runner.Result {
	ch := make(chan runner.Result, 1)
	ch <- runner.Result{Abort: true}
	return ch
}
