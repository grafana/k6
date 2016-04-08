package run

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/comm"
	"github.com/loadimpact/speedboat/runner"
	"github.com/loadimpact/speedboat/util"
	"github.com/loadimpact/speedboat/worker"
)

func init() {
	worker.RegisterProcessor(func(*worker.Worker) comm.Processor {
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

func (p *LoadTestProcessor) Process(msg comm.Message) <-chan comm.Message {
	ch := make(chan comm.Message)

	go func() {
		defer close(ch)

		switch msg.Type {
		case "test.run":
			data := MessageTestRun{}
			if err := msg.Take(&data); err != nil {
				ch <- comm.ToClient("error").WithError(err)
				return
			}
			for res := range p.ProcessRun(data) {
				ch <- res
			}
		case "test.scale":
			data := MessageTestScale{}
			if err := msg.Take(&data); err != nil {
				ch <- comm.ToClient("error").WithError(err)
				return
			}
			for res := range p.ProcessScale(data) {
				ch <- res
			}
		case "test.stop":
			for res := range p.ProcessStop(MessageTestStop{}) {
				ch <- res
			}
		}
	}()

	return ch
}

func (p *LoadTestProcessor) ProcessRun(data MessageTestRun) <-chan comm.Message {
	ch := make(chan comm.Message)

	go func() {
		defer close(ch)

		p.controlChannel = make(chan int, 1)
		p.currentVUs = data.VUs

		log.WithFields(log.Fields{
			"filename": data.Filename,
			"vus":      data.VUs,
		}).Debug("Running script")

		var r runner.Runner = nil

		r, err := util.GetRunner(data.Filename)
		if err != nil {
			ch <- comm.ToClient("error").WithError(err)
			return
		}

		err = r.Load(data.Filename, data.Source)
		if err != nil {
			ch <- comm.ToClient("error").WithError(err)
			return
		}

		p.controlChannel <- data.VUs
		for res := range runner.Run(r, p.controlChannel) {
			switch res := res.(type) {
			case runner.LogEntry:
				ch <- comm.ToClient("test.log").With(res)
			case runner.Metric:
				ch <- comm.ToClient("test.metric").With(res)
			case error:
				ch <- comm.ToClient("error").WithError(res)
			}
		}
	}()

	return ch
}

func (p *LoadTestProcessor) ProcessScale(data MessageTestScale) <-chan comm.Message {
	ch := make(chan comm.Message)

	go func() {
		defer close(ch)

		delta := data.VUs - p.currentVUs

		log.WithFields(log.Fields{
			"from":  p.currentVUs,
			"to":    data.VUs,
			"delta": delta,
		}).Debug("Scaling")

		p.controlChannel <- delta
		p.currentVUs = data.VUs
	}()

	return ch
}

func (p *LoadTestProcessor) ProcessStop(data MessageTestStop) <-chan comm.Message {
	ch := make(chan comm.Message)

	go func() {
		defer close(ch)

		close(p.controlChannel)
	}()

	return ch
}
