package common

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
)

type Worker struct {
	ctx     context.Context
	session session

	targetID target.ID
	url      string
}

// NewWorker creates a new worker and starts its event loop.
func NewWorker(ctx context.Context, s session, id target.ID, url string) (*Worker, error) {
	w := Worker{
		ctx:      ctx,
		session:  s,
		targetID: id,
		url:      url,
	}
	if err := w.initEvents(); err != nil {
		return nil, err
	}

	w.startEventLoop()

	return &w, nil
}

func (w *Worker) initEvents() error {
	actions := []Action{
		log.Enable(),
		network.Enable(),
		runtime.Enable(),
		runtime.RunIfWaitingForDebugger(),
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(w.ctx, w.session)); err != nil {
			return fmt.Errorf("protocol error while initializing worker %T: %w", action, err)
		}
	}
	return nil
}

// startEventLoop keeps the worker session alive by listening for events.
func (w *Worker) startEventLoop() {
	// Listening for [cdproto.EventRuntimeExecutionContextCreated] seems enough for now.
	eventCh := make(chan Event)
	w.session.on(w.ctx, []string{cdproto.EventRuntimeExecutionContextCreated}, eventCh)

	go func() {
		for {
			select {
			case <-w.session.Done():
				return
			case <-w.ctx.Done():
				return
			case event := <-eventCh:
				// This seems to allow the worker to establish connections on page reloads.
				if _, ok := event.data.(*runtime.EventExecutionContextCreated); ok {
					action := runtime.RunIfWaitingForDebugger()
					_ = action.Do(cdp.WithExecutor(w.ctx, w.session))
				}
			}
		}
	}()
}

// URL returns the URL of the web worker.
func (w *Worker) URL() string {
	return w.url
}
