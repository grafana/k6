package simple

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/proto/httpwrap"
	"github.com/loadimpact/speedboat/stats"
	"math"
	"net/http"
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
	Client    http.Client
	Request   http.Request
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
		Runner: r,
		Client: http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: math.MaxInt32,
			},
		},
		Request:   *req,
		Collector: stats.NewCollector(),
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	req := u.Request
	_, _, sample, err := httpwrap.Do(ctx, &u.Client, &req, httpwrap.Params{TakeSample: true})
	if err != nil {
		select {
		case <-ctx.Done():
			return nil
		default:
			log.WithError(err).Error("Request Error")
			u.Collector.Add(stats.Sample{
				Stat: &mErrors,
				Tags: stats.Tags{
					"method": req.Method,
					"url":    req.URL.String(),
					"error":  err.Error(),
				},
				Values: stats.Value(1),
			})
			return err
		}
	}

	sample.Stat = &mRequests
	u.Collector.Add(sample)

	return nil
}
