package dogstatsd

import (
	"github.com/loadimpact/k6/core/statsd"
	log "github.com/sirupsen/logrus"
)

// TagHandler defines a tag handler type
type TagHandler struct{}

// Process implements the interface method of Tagger
func (t *TagHandler) Process(whitelist string) func(map[string]string) []string {
	return func(tags map[string]string) []string {
		return statsd.MapToSlice(
			statsd.TakeOnly(tags, whitelist),
		)
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
