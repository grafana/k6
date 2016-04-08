package run

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	worker.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &LoadTestProcessor{}
	})
}

type LoadTestProcessor struct {
	// Write a positive number to this to spawn so many VUs, negative to kill
	// that many. Close it to kill all VUs and end the running test.
	controlChannel chan int

	// Counter for how many VUs we currently have running.
	currentVUs int
}

func (p *LoadTestProcessor) Process(msg message.Message) <-chan message.Message {
	ch := make(chan message.Message)

	go func() {
		defer close(ch)

		switch msg.Type {
		case "test.run":
			data := MessageTestRun{}
			if err := msg.Take(&data); err != nil {
				ch <- message.ToClient("error").WithError(err)
				return
			}

			p.controlChannel = make(chan int, 1)
			p.currentVUs = data.VUs

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

			p.controlChannel <- data.VUs
			for res := range runner.Run(r, p.controlChannel) {
				switch res := res.(type) {
				case runner.LogEntry:
					ch <- message.ToClient("test.log").With(res)
				case runner.Metric:
					ch <- message.ToClient("test.metric").With(res)
				case error:
					ch <- message.ToClient("error").WithError(res)
				}
			}
		case "test.scale":
			data := MessageTestScale{}
			if err := msg.Take(&data); err != nil {
				ch <- message.ToClient("error").WithError(err)
				return
			}

			delta := data.VUs - p.currentVUs
			log.WithFields(log.Fields{
				"from":  p.currentVUs,
				"to":    data.VUs,
				"delta": delta,
			}).Debug("Scaling")
			p.controlChannel <- delta
			p.currentVUs = data.VUs
		case "test.stop":
			close(p.controlChannel)
		}
	}()

	return ch
}
