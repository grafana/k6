package cloudlog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils"
)

// TestPusher is the canonical case: entries fired before and after SetConfig
// are buffered and then pushed to the configured endpoint, carrying the
// bearer token and the required test_run_id label.
func TestPusher(t *testing.T) {
	t.Parallel()

	type received struct {
		body string
		auth string
	}
	recvCh := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		select {
		case recvCh <- received{body: string(b), auth: req.Header.Get("Authorization")}:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := New(testutils.NewLogger(t))

	// Fired BEFORE SetConfig: must be buffered until the pusher is configured.
	require.NoError(t, p.Fire(&logrus.Entry{Time: time.Now(), Level: logrus.InfoLevel, Message: "before-config"}))

	p.SetConfig(Config{
		PushURL:    srv.URL,
		Token:      "test-token",
		TestRunID:  "run-123",
		PushPeriod: 20 * time.Millisecond,
	})

	// Fired AFTER SetConfig: still buffered until Listen drains.
	require.NoError(t, p.Fire(&logrus.Entry{Time: time.Now(), Level: logrus.InfoLevel, Message: "after-config"}))

	ctx, cancel := context.WithCancel(context.Background())
	listenDone := make(chan struct{})
	go func() {
		p.Listen(ctx)
		close(listenDone)
	}()

	var got received
	select {
	case got = <-recvCh:
	case <-time.After(2 * time.Second):
		t.Fatal("no push received from the pusher")
	}

	assert.Equal(t, "Bearer test-token", got.auth)
	assert.Contains(t, got.body, "before-config")
	assert.Contains(t, got.body, "after-config")
	assert.Contains(t, got.body, `"test_run_id":"run-123"`)

	cancel()
	select {
	case <-listenDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Listen did not return after ctx was cancelled")
	}
}

// TestPusher_UnconfiguredDrainsOnCtxDone covers the --no-cloud-logs / early
// exit path: without SetConfig, Listen returns promptly on ctx.Done and never
// pushes.
func TestPusher_UnconfiguredDrainsOnCtxDone(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Errorf("pusher made an HTTP request while unconfigured")
	}))
	defer srv.Close()

	p := New(testutils.NewLogger(t))
	require.NoError(t, p.Fire(&logrus.Entry{Time: time.Now(), Level: logrus.InfoLevel, Message: "buffered"}))

	ctx, cancel := context.WithCancel(context.Background())
	listenDone := make(chan struct{})
	go func() {
		p.Listen(ctx)
		close(listenDone)
	}()

	cancel()
	select {
	case <-listenDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Listen did not return promptly on ctx.Done while unconfigured")
	}
}

// TestPusher_FireNeverBlocks keeps the logging path non-blocking: with no
// Listen draining, Fire returns immediately even past the buffer capacity and
// over-cap entries are counted as dropped.
func TestPusher_FireNeverBlocks(t *testing.T) {
	t.Parallel()

	p := New(testutils.NewLogger(t))

	const overflow = 10
	total := bufferCap + overflow

	fired := make(chan struct{})
	go func() {
		for range total {
			_ = p.Fire(&logrus.Entry{Time: time.Now(), Level: logrus.InfoLevel, Message: "x"})
		}
		close(fired)
	}()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("Fire blocked when the buffer was full")
	}

	assert.Equal(t, int64(overflow), p.dropped.Load())
}

// TestPusher_TestRunIDLabelKept locks the backend's hard requirement: even
// when AllowedLabels omits test_run_id, the pushed stream still carries it as
// a label (the backend 401s a push whose stream lacks it).
func TestPusher_TestRunIDLabelKept(t *testing.T) {
	t.Parallel()

	recvCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		select {
		case recvCh <- string(b):
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	p := New(testutils.NewLogger(t))
	p.SetConfig(Config{
		PushURL:       srv.URL,
		Token:         "tok",
		TestRunID:     "run-xyz",
		PushPeriod:    20 * time.Millisecond,
		AllowedLabels: []string{"level"}, // deliberately omits test_run_id
	})
	require.NoError(t, p.Fire(&logrus.Entry{Time: time.Now(), Level: logrus.InfoLevel, Message: "hello"}))

	ctx, cancel := context.WithCancel(context.Background())
	listenDone := make(chan struct{})
	go func() {
		p.Listen(ctx)
		close(listenDone)
	}()

	select {
	case body := <-recvCh:
		assert.Contains(t, body, `"test_run_id":"run-xyz"`)
	case <-time.After(2 * time.Second):
		t.Fatal("no push received from the pusher")
	}

	cancel()
	<-listenDone
}
