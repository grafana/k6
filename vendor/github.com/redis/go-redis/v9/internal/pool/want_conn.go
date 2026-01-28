package pool

import (
	"context"
	"sync"
)

type wantConn struct {
	mu        sync.Mutex      // protects ctx, done and sending of the result
	ctx       context.Context // context for dial, cleared after delivered or canceled
	cancelCtx context.CancelFunc
	done      bool                // true after delivered or canceled
	result    chan wantConnResult // channel to deliver connection or error
}

// getCtxForDial returns context for dial or nil if connection was delivered or canceled.
func (w *wantConn) getCtxForDial() context.Context {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.ctx
}

func (w *wantConn) tryDeliver(cn *Conn, err error) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done {
		return false
	}

	w.done = true
	w.ctx = nil

	w.result <- wantConnResult{cn: cn, err: err}
	close(w.result)

	return true
}

func (w *wantConn) cancel() *Conn {
	w.mu.Lock()
	var cn *Conn
	if w.done {
		select {
		case result := <-w.result:
			cn = result.cn
		default:
		}
	} else {
		close(w.result)
	}

	w.done = true
	w.ctx = nil
	w.mu.Unlock()

	return cn
}

type wantConnResult struct {
	cn  *Conn
	err error
}

type wantConnQueue struct {
	mu    sync.RWMutex
	items []*wantConn
}

func newWantConnQueue() *wantConnQueue {
	return &wantConnQueue{
		items: make([]*wantConn, 0),
	}
}

func (q *wantConnQueue) enqueue(w *wantConn) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, w)
}

func (q *wantConnQueue) dequeue() (*wantConn, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return nil, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}
