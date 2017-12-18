package statsd

import (
	statsd "github.com/loadimpact/k6/stats/statsd/common"
	log "github.com/sirupsen/logrus"
)

// TagHandler defines a tag handler type
type TagHandler struct{}

// Process implements the interface method of Tagger
func (t *TagHandler) Process(whitelist string) func(map[string]string, string) []string {
	return func(tags map[string]string, group string) []string {
		return []string{}
	}
}

// New creates a new statsd connector client
func New(conf statsd.Config) (*statsd.Collector, error) {
	cl, err := statsd.MakeClient(conf, statsd.StatsD)
	if err != nil {
		return nil, err
	}

	return &statsd.Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", statsd.StatsD.String()),
		Type:   statsd.StatsD,
		Tagger: &TagHandler{},
	}, nil
}
