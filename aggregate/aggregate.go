package aggregate

import (
	"github.com/loadimpact/speedboat/runner"
)

func Aggregate(stats *Stats, in <-chan runner.Result) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		for res := range in {
			stats.Ingest(&res)
			ch <- res
		}

		stats.End()
	}()

	return ch
}
