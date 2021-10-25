/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/log"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"

	"golang.org/x/net/context"
)

// Ensure Worker implements the EventEmitter, Target and api.Worker interfaces
var _ EventEmitter = &Worker{}
var _ api.Worker = &Worker{}

// Worker represents a WebWorker
type Worker struct {
	BaseEventEmitter

	ctx     context.Context
	session *Session

	targetID target.ID
	url      string
}

// NewWorker creates a new page viewport
func NewWorker(ctx context.Context, session *Session, id target.ID, url string) (*Worker, error) {
	w := Worker{
		BaseEventEmitter: NewBaseEventEmitter(),
		ctx:              ctx,
		session:          session,
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
			return fmt.Errorf("unable to execute %T: %v", action, err)
		}
	}
	return nil
}

// Evaluate evaluates a page function in the context of the web worker
func (w *Worker) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	// TODO: implement
	return nil
}

// EvaluateHandle evaluates a page function in the context of the web worker and returns a JS handle
func (w *Worker) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	// TODO: implement
	return nil
}

// URL returns the URL of the web worker
func (w *Worker) URL() string {
	return w.url
}
