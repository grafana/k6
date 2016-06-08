package speedboat

import (
	"golang.org/x/net/context"
)

const (
	AbortTest FlowControl = 0
)

type FlowControl int

func (op FlowControl) Error() string {
	switch op {
	case 0:
		return "OP: Abort Test"
	default:
		return "Unknown flow control OP"
	}
}

type Runner interface {
	RunVU(ctx context.Context, t Test, id int)
}

/*type Result struct {
	Text  string
	Time  time.Duration
	Extra map[string]interface{}
	Error error
	Abort bool
}

type VU struct {
	Cancel context.CancelFunc
}

func Run(ctx context.Context, r Runner, t loadtest.LoadTest, scale <-chan int) <-chan Result {
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
						for res := range r.Run(c, t, currentID) {
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

func Ramp(t *loadtest.LoadTest, scale chan int, in <-chan Result) <-chan Result {
	ch := make(chan Result)

	go func() {
		defer close(ch)

		ticker := time.NewTicker(time.Duration(1) * time.Second)
		startTime := time.Now()
		for {
			select {
			case <-ticker.C:
				vus, _ := t.VUsAt(time.Since(startTime))
				scale <- vus
			case res, ok := <-in:
				if !ok {
					return
				}
				ch <- res
			}
		}
	}()

	return ch
}*/
