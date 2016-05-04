package aggregate

import (
	"github.com/loadimpact/speedboat/runner"
)

func Aggregate(stats *Stats, in <-chan runner.Result) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		defer stats.End()
		for res := range in {
			if res.Abort {
				continue
			}
			stats.Ingest(&res)
			ch <- res
		}
	}()

	return ch
}
