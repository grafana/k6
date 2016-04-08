package ping

import (
	"github.com/codegangsta/cli"
	"github.com/loadimpact/speedboat/comm"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	master.RegisterProcessor(func(*master.Master) comm.Processor {
		return &PingProcessor{}
	})
	worker.RegisterProcessor(func(*worker.Worker) comm.Processor {
		return &PingProcessor{}
	})
}

// Processes pings, on both master and worker.
type PingProcessor struct{}

func (*PingProcessor) Process(msg comm.Message) <-chan comm.Message {
	out := make(chan comm.Message)

	go func() {
		defer close(out)
		switch msg.Type {
		case "ping.ping":
			data := PingMessage{}
			if err := msg.Take(&data); err != nil {
				out <- comm.ToClient("error").WithError(err)
				break
			}
			out <- comm.ToClient("ping.pong").With(data)
		}
	}()

	return out
}
