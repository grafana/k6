package json

import (
	"context"
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/stats"
	"io"
	"net/url"
	"os"
)

type Collector struct {
	outfile io.WriteCloser
	fname   string
	types   []string
}

func (c *Collector) InTypeList(str string) bool {
	for _, n := range c.types {
		if n == str {
			return true
		}
	}
	return false
}

func New(u *url.URL) (*Collector, error) {
	fname := u.Path
	if u.Path == "" {
		fname = u.String()
	}

	logfile, err := os.Create(fname)
	if err != nil {
		return nil, err
	}

	t := make([]string, 16)
	return &Collector{
		outfile: logfile,
		fname:   fname,
		types:   t,
	}, nil
}

func (c *Collector) String() string {
	return "JSON"
}

func (c *Collector) Run(ctx context.Context) {
	log.WithField("filename", c.fname).Debug("JSON: Writing JSON metrics")
	<-ctx.Done()
	err := c.outfile.Close()
	if err == nil {
		return
	}
}

func (c *Collector) Collect(samples []stats.Sample) {
	for _, sample := range samples {
		if !c.InTypeList(sample.Metric.Name) {
			c.types = append(c.types, sample.Metric.Name)
			if env := Wrap(sample.Metric); env != nil {
				row, err := json.Marshal(env)
				if err == nil {
					row = append(row, '\n')
					_, err := c.outfile.Write(row)
					if err != nil {
						log.WithField("filename", c.fname).Error("JSON: Error writing to file")
					}

				}
			}

		}

		env := Wrap(sample)
		row, err := json.Marshal(env)
		if err != nil || env == nil {
			// Skip metric if it can't be made into JSON or envelope is null.
			continue
		}
		row = append(row, '\n')
		_, err = c.outfile.Write(row)
		if err != nil {
			log.WithField("filename", c.fname).Error("JSON: Error writing to file")
		}
	}
}
