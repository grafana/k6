package websockets

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Go tip on Windows keeps a pooled worker for synchronous file I/O alive
	// after the tests finish. It belongs to the runtime, not to this package.
	goleak.VerifyTestMain(m, goleak.IgnoreTopFunction("internal/poll.(*syncIOWorkerState).run"))
}
