package tests

import (
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"
)

type blockingTransport struct {
	fallback       http.RoundTripper
	forbiddenHosts map[string]bool
	counter        uint32
}

func (bt *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if bt.forbiddenHosts[host] {
		atomic.AddUint32(&bt.counter, 1)
		panic(fmt.Errorf("trying to make forbidden request to %s during test", host))
	}
	return bt.fallback.RoundTrip(req)
}

// Main is a TestMain function that can be imported by other test packages that
// want to use the blocking transport and other features useful for integration
// tests.
func Main(m *testing.M) {
	exitCode := 1 // error out by default
	defer func() {
		os.Exit(exitCode)
	}()

	bt := &blockingTransport{
		fallback: http.DefaultTransport,
		forbiddenHosts: map[string]bool{
			"ingest.k6.io":    true,
			"cloudlogs.k6.io": true,
			"app.k6.io":       true,
			"reports.k6.io":   true,
		},
	}
	http.DefaultTransport = bt
	defer func() {
		if bt.counter > 0 {
			fmt.Printf("Expected blocking transport count to be 0 but was %d\n", bt.counter) //nolint:forbidigo
			exitCode = 2
		}
	}()

	defer func() {
		// TODO: figure out why logrus' `Entry.WriterLevel` goroutine sticks
		// around and remove this exception.
		opt := goleak.IgnoreTopFunction("io.(*pipe).read")
		if err := goleak.Find(opt); err != nil {
			fmt.Println(err) //nolint:forbidigo
			exitCode = 3
		}
	}()

	exitCode = m.Run()
}
