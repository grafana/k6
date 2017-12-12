package statsd

import log "github.com/sirupsen/logrus"

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

// Possible values for ClientType.
const (
	StatsD = ClientType(iota)
	DogStatsD
)

// NewStatsD initialises a new StatsD collector
func NewStatsD(conf Config) (*Collector, error) {
	cl, err := MakeClient(conf, StatsD)
	if err != nil {
		return nil, err
	}
	return &Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", StatsD.String()),
		Type:   StatsD,
	}, nil
}

// NewDogStatsD initialises a new DogStatsD collector
func NewDogStatsD(conf Config) (*Collector, error) {
	cl, err := MakeClient(conf, DogStatsD)
	if err != nil {
		return nil, err
	}
	return &Collector{
		Client: cl,
		Config: conf,
		Logger: log.WithField("type", DogStatsD.String()),
		Type:   DogStatsD,
	}, nil
}
