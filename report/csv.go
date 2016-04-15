package report

import (
	"fmt"
	"github.com/loadimpact/speedboat/runner"
	"io"
	"time"
)

type CSVReporter struct{}

func (CSVReporter) Begin(w io.Writer) {}

func (CSVReporter) Report(w io.Writer, res *runner.Result) {
	// TODO: Timestamp events themselves!
	t := time.Now()
	errString := ""
	if res.Error != nil {
		errString = res.Error.Error()
	}
	fmt.Fprintf(w, "%d;%d;%s;%s\n", t.Unix(), res.Time.Nanoseconds(), res.Text, errString)
}

func (CSVReporter) End(w io.Writer) {}
