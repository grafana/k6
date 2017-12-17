package dogstatsd

import (
	"fmt"

	statsd "github.com/loadimpact/k6/stats/statsd/shared"
	log "github.com/sirupsen/logrus"
)

// TagHandler defines a tag handler type
type TagHandler struct{}

// Process implements the interface method of Tagger
func (t *TagHandler) Process(whitelist string) func(map[string]string, string) []string {
	return func(tags map[string]string, group string) []string {
		slice := statsd.MapToSlice(
			statsd.TakeOnly(tags, whitelist),
		)
		return append(slice, fmt.Sprintf("group:%s", group))
	}
}

// New creates a new statsd connector client
func New(conf statsd.Config) (*statsd.Collector, error) {
	tagHandler := &TagHandler{}
	cl, err := statsd.MakeClient(conf, statsd.DogStatsD)
	if err != nil {
		return nil, err
	}

	if namespace := conf.Extra().Namespace; namespace != "" {
		cl.Namespace = namespace
	}

	return &statsd.Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", statsd.DogStatsD.String()),
		Type:   statsd.DogStatsD,
		Tagger: tagHandler,
	}, nil
}
