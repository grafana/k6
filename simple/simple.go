package simple

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"time"
)

type Runner struct {
	Test lib.Test

	mDuration *sampler.Metric
	mErrors   *sampler.Metric
}

type VU struct {
	Runner  *Runner
	Client  fasthttp.Client
	Request fasthttp.Request
}

func New(t lib.Test) *Runner {
	return &Runner{
		Test:      t,
		mDuration: sampler.Stats("request.duration"),
		mErrors:   sampler.Counter("request.error"),
	}
}

func (r *Runner) NewVU() (lib.VU, error) {
	vu := &VU{
		Runner: r,
		Client: fasthttp.Client{MaxConnsPerHost: math.MaxInt32},
	}

	vu.Request.SetRequestURI(r.Test.URL)

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

	u.Runner.mDuration.WithFields(sampler.Fields{
		"url":    u.Runner.Test.URL,
		"method": "GET",
		"status": res.StatusCode(),
	}).Duration(duration)

	if err != nil {
		log.WithError(err).Error("Request error")
		u.Runner.mErrors.WithFields(sampler.Fields{
			"url":    u.Runner.Test.URL,
			"method": "GET",
			"status": res.StatusCode(),
		}).Int(1)
		return err
	}

	return nil
}
