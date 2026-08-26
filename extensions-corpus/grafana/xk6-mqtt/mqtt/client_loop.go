package mqtt

import "github.com/mstoykov/k6-taskqueue-lib/taskqueue"

func (c *client) loop() {
	ctx := c.vu.Context()
	tq := taskqueue.New(c.vu.RegisterCallback)

	defer tq.Close()
	defer c.stopLoop()

	for {
		select {
		case call := <-c.callChan:
			tq.Queue(call)
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}
