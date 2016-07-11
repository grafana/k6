package simple

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
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
	Client    fasthttp.Client
	Request   fasthttp.Request
	Collector *stats.Collector
}

func New(url string) *Runner {
	return &Runner{
		URL: url,
	}
}

func (r *Runner) NewVU() (lib.VU, error) {
	vu := &VU{
		Runner:    r,
		Client:    fasthttp.Client{MaxConnsPerHost: math.MaxInt32},
		Collector: stats.NewCollector(),
	}

	vu.Request.SetRequestURI(r.URL)

	return vu, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	startTime := time.Now()
	err := u.Client.Do(&u.Request, res)
	duration := time.Since(startTime)

	tags := stats.Tags{
		"url":    u.Runner.URL,
		"method": "GET",
		"status": res.StatusCode(),
	}
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
