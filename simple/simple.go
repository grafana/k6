package simple

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"golang.org/x/net/context"
	"net/http"
	"time"
)

var (
	mRequests = stats.Stat{Name: "requests", Type: stats.HistogramType, Intent: stats.TimeIntent}
	mErrors   = stats.Stat{Name: "errors", Type: stats.CounterType}
)

type Runner struct {
	URL string
}

type VU struct {
	Runner    *Runner
	Client    *http.Client
	Request   *http.Request
	Collector *stats.Collector
}

func New(url string) *Runner {
	return &Runner{
		URL: url,
	}
}

func (r *Runner) NewVU() (lib.VU, error) {
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		return nil, err
	}

	return &VU{
		Runner:    r,
		Client:    &http.Client{},
		Request:   req,
		Collector: stats.NewCollector(),
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	startTime := time.Now()
	res, err := u.Client.Do(u.Request)
	duration := time.Since(startTime)

	status := 0
	if err == nil {
		status = res.StatusCode
		res.Body.Close()
	}

	tags := stats.Tags{"method": "GET", "url": u.Runner.URL, "status": status}
	u.Collector.Add(stats.Sample{
		Stat:   &mRequests,
		Tags:   tags,
		Values: stats.Values{"duration": float64(duration)},
	})

	if err != nil {
		log.WithError(err).Error("Request error")
		u.Collector.Add(stats.Sample{
			Stat:   &mErrors,
			Tags:   tags,
			Values: stats.Value(1),
		})
		return err
	}

	return nil
}
