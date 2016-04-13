package runner

import (
	"golang.org/x/net/context"
	"sync"
	"time"
)

type Runner interface {
	Run(ctx context.Context) <-chan Result
}

type Result struct {
	Text  string
	Time  time.Duration
	Error error
}

type VU struct {
	Cancel context.CancelFunc
}

func Run(ctx context.Context, r Runner, scale <-chan int) <-chan Result {
	ch := make(chan Result)

	go func() {
		defer close(ch)

		currentVUs := []VU{}
		wg := sync.WaitGroup{}
		for {
			select {
			case vus := <-scale:
				for vus > len(currentVUs) {
					wg.Add(1)
					c, cancel := context.WithCancel(ctx)
					currentVUs = append(currentVUs, VU{Cancel: cancel})
					go func() {
						defer wg.Done()
						for res := range r.Run(c) {
							ch <- res
						}
					}()
				}
				for vus < len(currentVUs) {
					currentVUs[len(currentVUs)-1].Cancel()
					currentVUs = currentVUs[:len(currentVUs)-1]
				}
			case <-ctx.Done():
				return
			}
		}

		wg.Wait()
	}()

	return ch
}
