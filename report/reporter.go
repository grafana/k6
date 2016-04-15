package report

import (
	"github.com/loadimpact/speedboat/runner"
	"io"
)

type Reporter interface {
	Begin(w io.Writer)
	Report(w io.Writer, res *runner.Result)
	End(w io.Writer)
}

func Report(r Reporter, w io.Writer, in <-chan runner.Result) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		r.Begin(w)
		for res := range in {
			r.Report(w, &res)
			ch <- res
		}
		r.End(w)
	}()

	return ch
}
