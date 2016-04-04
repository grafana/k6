package loadtest

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	registry.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &LoadTestProcessor{}
	})
}

type LoadTestProcessor struct {
	// Close this channel to stop the currently running test
	stopChannel chan interface{}
}

func (p *LoadTestProcessor) Process(msg message.Message) <-chan message.Message {
	ch := make(chan message.Message)

	go func() {
		defer close(ch)

		switch msg.Type {
		case "test.run":
			p.stopChannel = make(chan interface{})

			data := MessageTestRun{}
			if err := msg.Take(&data); err != nil {
				ch <- message.ToClient("error").WithError(err)
				return
			}

			log.WithFields(log.Fields{
				"filename": data.Filename,
				"vus":      data.VUs,
			}).Debug("Running script")

			var r runner.Runner = nil

			r, err := js.New()
			if err != nil {
				ch <- message.ToClient("error").WithError(err)
				break
			}

			err = r.Load(data.Filename, data.Source)
			if err != nil {
				ch <- message.ToClient("error").WithError(err)
				break
			}

			for res := range runner.Run(r, data.VUs, p.stopChannel) {
				switch res := res.(type) {
				case runner.LogEntry:
					ch <- message.NewToClient("run.log", message.Fields{
						"text": res.Text,
					})
				case runner.Metric:
					ch <- message.NewToClient("run.metric", message.Fields{
						"start":    res.Start,
						"duration": res.Duration,
					})
				case error:
					ch <- message.NewToClient("run.error", message.Fields{
						"error": res.Error(),
					})
				}
			}
		case "test.stop":
			close(p.stopChannel)
		}
	}()

	return ch
}
