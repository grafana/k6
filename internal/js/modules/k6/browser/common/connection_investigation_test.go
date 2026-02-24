package common

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/internal/js/modules/k6/browser/tests/ws"
)

// Ensure the connection can tear down even when an abnormal closure happens
// while there is no request actively waiting on c.errorCh.
func TestConnectionAbnormalClosureIdleCloses(t *testing.T) {
	t.Parallel()

	server := ws.NewServer(t, ws.WithClosureAbnormalHandler("/closure-abnormal-idle"))

	ctx := context.Background()
	u, _ := url.Parse(server.ServerHTTP.URL)
	wsURL := fmt.Sprintf("ws://%s/closure-abnormal-idle", u.Host)

	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	t.Cleanup(conn.Close)

	select {
	case <-conn.done:
		// Expected.
	case <-time.After(3 * time.Second):
		t.Fatalf("connection.done did not close after idle abnormal closure")
	}
}

func TestConnectionAbnormalClosureWithPendingRequestCloses(t *testing.T) {
	t.Parallel()

	server := ws.NewServer(t, ws.WithClosureAbnormalHandler("/closure-abnormal-pending"))

	ctx := context.Background()
	u, _ := url.Parse(server.ServerHTTP.URL)
	wsURL := fmt.Sprintf("ws://%s/closure-abnormal-pending", u.Host)

	conn, err := NewConnection(ctx, wsURL, log.NewNullLogger(), nil)
	if err != nil {
		t.Fatalf("new connection: %v", err)
	}
	t.Cleanup(conn.Close)

	err = target.SetDiscoverTargets(true).Do(cdp.WithExecutor(ctx, conn))
	if err == nil {
		t.Fatalf("expected abnormal-closure error")
	}

	select {
	case <-conn.done:
		// Expected: once a sender is receiving from errorCh/closeCh, teardown completes.
	case <-time.After(3 * time.Second):
		t.Fatalf("connection.done did not close with pending request")
	}
}
