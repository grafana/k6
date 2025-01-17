package common

import (
	"context"
	"fmt"

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

// NewWorker creates a new page viewport.
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

	return &w, nil
}

func (w *Worker) initEvents() error {
	actions := []Action{
		log.Enable(),
		network.Enable(),
		runtime.RunIfWaitingForDebugger(),
	}
	for _, action := range actions {
		if err := action.Do(cdp.WithExecutor(w.ctx, w.session)); err != nil {
			return fmt.Errorf("protocol error while initializing worker %T: %w", action, err)
		}
	}
	return nil
}

// URL returns the URL of the web worker.
func (w *Worker) URL() string {
	return w.url
}
