package jsonlog

import (
	"context"
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/stats"
	"net/url"
	"os"
	"time"
)

type Collector struct {
	f     *os.File
	types map[string]stats.Metric
}

func New(u *url.URL) (*Collector, error) {
	var fname string

	if u.Path == "" {
		fname = u.String()
	} else {
		fname = u.Path
	}

	logfile, err := os.Create(fname)
	if err != nil {
		return nil, err
	}

	return &Collector{
		f:     logfile,
		types: map[string]stats.Metric{},
	}, nil
}

func (c *Collector) String() string {
	return "jsonlog"
}

func (c *Collector) Run(ctx context.Context) {
	log.Debug("Writing metrics as JSON to ", c.f.Name())
	for {
		select {
		case <-ctx.Done():
			c.writeTypes()
			c.f.Close()
			return
		}
	}
}

func (c *Collector) writeTypes() {
	types, err := json.Marshal(c.types)
	if err != nil {
		return
	}
	c.f.WriteString(string(types) + "\n")
}

func (c *Collector) Collect(samples []stats.Sample) {
	for _, sample := range samples {
		if _, present := c.types[sample.Metric.Name]; !present {
			c.types[sample.Metric.Name] = *sample.Metric
		}

		row, err := json.Marshal(NewJSONPoint(&sample))
		if err != nil {
			// Skip metric if it can't be made into JSON.
			continue
		}
		c.f.WriteString(string(row) + "\n")
	}
}

type JSONPoint struct {
	Type  string            `json:"type"`
	Time  time.Time         `json:"timestamp"`
	Value float64           `json:"value"`
	Tags  map[string]string `json:"tags"`
}

func NewJSONPoint(sample *stats.Sample) *JSONPoint {
	return &JSONPoint{
		Type:  sample.Metric.Name,
		Time:  sample.Time,
		Value: sample.Value,
		Tags:  sample.Tags,
	}
}
