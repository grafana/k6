package common

import (
	"context"
	"fmt"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6error"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
)

// Ensure Worker implements the EventEmitter, Target and api.Worker interfaces.
var _ EventEmitter = &Worker{}
var _ api.Worker = &Worker{}

type Worker struct {
	BaseEventEmitter

	ctx     context.Context
	session session

	targetID target.ID
	url      string
}

// NewWorker creates a new page viewport.
func NewWorker(ctx context.Context, s session, id target.ID, url string) (*Worker, error) {
	w := Worker{
		BaseEventEmitter: NewBaseEventEmitter(ctx),
		ctx:              ctx,
		session:          s,
		targetID:         id,
		url:              url,
	}
	if err := w.initEvents(); err != nil {
		return nil, err
	}

	return &w, nil
}

func (w *Worker) didClose() {
	w.emit(EventWorkerClose, w)
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

// Evaluate evaluates a page function in the context of the web worker.
func (w *Worker) Evaluate(pageFunc goja.Value, args ...goja.Value) any {
	// TODO: implement
	return nil
}

// EvaluateHandle evaluates a page function in the context of the web worker and returns a JS handle.
func (w *Worker) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (api.JSHandle, error) {
	// TODO: implement
	return nil, fmt.Errorf("Worker.EvaluateHandle has not been implemented yet: %w", k6error.ErrFatal)
}

// URL returns the URL of the web worker.
func (w *Worker) URL() string {
	return w.url
}
