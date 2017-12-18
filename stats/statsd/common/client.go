package statsd

import (
	"fmt"

	"github.com/DataDog/datadog-go/statsd"
	log "github.com/sirupsen/logrus"
)

// ClientType defines a statsd client type
type ClientType int

func (t ClientType) String() string {
	switch t {
	case StatsD:
		return "StatsD"
	case DogStatsD:
		return "DogStatsD"
	default:
		return "[INVALID]"
	}
}

// Possible values for ClientType
const (
	StatsD = ClientType(iota)
	DogStatsD
	connStrSplitter = ":"
)

// MakeClient creates a new statsd buffered generic client
func MakeClient(conf Config, cliType ClientType) (*statsd.Client, error) {
	if conf.Address() == "" || conf.Port() == "" {
		return nil, fmt.Errorf(
			"%s: connection string is invalid. Received: \"%+s%s%s\"",
			cliType, conf.Address(), connStrSplitter, conf.Port(),
		)
	}

	connStr := fmt.Sprintf("%s%s%s", conf.Address(), connStrSplitter, conf.Port())
	c, err := statsd.NewBuffered(connStr, conf.BufferSize())
	if err != nil {
		log.Info(err)
		return nil, err
	}

	return c, nil
}
