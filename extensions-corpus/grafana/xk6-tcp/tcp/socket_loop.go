package tcp

import (
	"context"
)

func (s *socket) loop(ctx context.Context) {
	s.log.Debug("Starting event loop")

	for {
		select {
		case call := <-s.callChan:
			callbackRegistrar(s.vu)()(call)
		case <-ctx.Done():
			s.log.Debug("Socket context done, stopping event loop")

			return
		}
	}
}
