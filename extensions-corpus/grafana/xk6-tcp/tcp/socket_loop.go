package tcp

import (
	"context"

	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
)

func (s *socket) loop(ctx context.Context) {
	tq := taskqueue.New(s.vu.RegisterCallback)

	defer tq.Close()

	s.log.Debug("Starting event loop")

	for {
		select {
		case call := <-s.callChan:
			tq.Queue(call)
		case <-ctx.Done():
			s.log.Debug("Socket context done, stopping event loop")

			return
		}
	}
}
