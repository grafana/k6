package stackless

import (
	"runtime"
	"sync"
)

// NewFunc returns stackless wrapper for the function f.
//
// Unlike f, the returned stackless wrapper doesn't use stack space
// on the goroutine that calls it.
// The wrapper may save a lot of stack space if the following conditions
// are met:
//
//   - f doesn't contain blocking calls on network, I/O or channels;
//   - f uses a lot of stack space;
//   - the wrapper is called from high number of concurrent goroutines.
//
// The stackless wrapper returns false if the call cannot be processed
// at the moment due to high load.
func NewFunc(f func(ctx interface{})) func(ctx interface{}) bool {
	if f == nil {
		// developer sanity-check
		panic("BUG: f cannot be nil")
	}

	funcWorkCh := make(chan *funcWork, runtime.GOMAXPROCS(-1)*2048)
	onceInit := func() {
		n := runtime.GOMAXPROCS(-1)
		for i := 0; i < n; i++ {
			go funcWorker(funcWorkCh, f)
		}
	}
	var once sync.Once

	return func(ctx interface{}) bool {
		once.Do(onceInit)
		fw := getFuncWork()
		fw.ctx = ctx

		select {
		case funcWorkCh <- fw:
		default:
			putFuncWork(fw)
			return false
		}
		<-fw.done
		putFuncWork(fw)
		return true
	}
}

func funcWorker(funcWorkCh <-chan *funcWork, f func(ctx interface{})) {
	for fw := range funcWorkCh {
		f(fw.ctx)
		fw.done <- struct{}{}
	}
}

func getFuncWork() *funcWork {
	v := funcWorkPool.Get()
	if v == nil {
		v = &funcWork{
			done: make(chan struct{}, 1),
		}
	}
	return v.(*funcWork)
}

func putFuncWork(fw *funcWork) {
	fw.ctx = nil
	funcWorkPool.Put(fw)
}

var funcWorkPool sync.Pool

type funcWork struct {
	ctx  interface{}
	done chan struct{}
}
