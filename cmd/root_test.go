package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/testutils"
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

func TestMain(m *testing.M) {
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

func TestDeprecatedOptionWarning(t *testing.T) {
	t.Parallel()

	ts := state.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "--logformat", "json", "run", "-"}
	ts.Stdin = bytes.NewBuffer([]byte(`
		console.log('foo');
		export default function() { console.log('bar'); };
	`))

	newRootCommand(ts.GlobalState).execute()

	logMsgs := ts.LoggerHook.Drain()
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "foo"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "bar"))
	assert.Contains(t, ts.Stderr.String(), `"level":"info","msg":"foo","source":"console"`)
	assert.Contains(t, ts.Stderr.String(), `"level":"info","msg":"bar","source":"console"`)

	// TODO: after we get rid of cobra, actually emit this message to stderr
	// and, ideally, through the log, not just print it...
	assert.False(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "logformat"))
	assert.Contains(t, ts.Stdout.String(), `--logformat has been deprecated`)
}
