package runner

import (
	"golang.org/x/net/context"
	"sync"
	"time"
)

type Runner interface {
	Run(ctx context.Context, id int64) <-chan Result
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
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
			close(ch)
		}()

		currentVUs := make([]VU, 0, 100)
		currentID := int64(0)
		for {
			select {
			case vus := <-scale:
				for vus > len(currentVUs) {
					currentID += 1
					currentID := currentID
					wg.Add(1)
					c, cancel := context.WithCancel(ctx)
					currentVUs = append(currentVUs, VU{Cancel: cancel})
					go func() {
						defer wg.Done()
						for res := range r.Run(c, currentID) {
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
	}()

	return ch
}
