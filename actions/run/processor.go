package run

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/message"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/runner/js"
	"github.com/loadimpact/speedboat/worker"
	"time"
)

func init() {
	registry.RegisterProcessor(func(*worker.Worker) master.Processor {
		return &RunProcessor{}
	})
}

type RunProcessor struct{}

func (p *RunProcessor) Process(msg message.Message) <-chan message.Message {
	ch := make(chan message.Message)

	go func() {
		defer func() {
			ch <- message.NewToClient("run.end", message.Fields{})
			close(ch)
		}()

		switch msg.Type {
		case "run.run":
			filename := msg.Fields["filename"].(string)
			src := msg.Fields["src"].(string)
			vus := int(msg.Fields["vus"].(float64))
			duration := time.Duration(msg.Fields["duration"].(float64)) * time.Millisecond

			log.WithFields(log.Fields{
				"filename": filename,
				"vus":      vus,
				"duration": duration,
			}).Debug("Running script")

			var r runner.Runner = nil

			r, err := js.New()
			if err != nil {
				ch <- message.NewToClient("run.error", message.Fields{"error": err})
				break
			}

			err = r.Load(filename, src)
			if err != nil {
				ch <- message.NewToClient("run.error", message.Fields{"error": err})
				break
			}

			for res := range runner.Run(r, vus, duration) {
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
		}
	}()

	return ch
}
