package simple

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"time"
)

type Runner struct {
}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) RunVU(ctx context.Context, t speedboat.Test, id int) {
	mDuration := sampler.Stats("request.duration")
	mErrors := sampler.Counter("request.error")

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	client := fasthttp.Client{MaxConnsPerHost: math.MaxInt32}

	for {
		req.Reset()
		req.SetRequestURI(t.URL)
		res.Reset()

		startTime := time.Now()
		if err := client.Do(req, res); err != nil {
			log.WithError(err).Error("Request error")
			mErrors.WithField("url", t.URL).Int(1)
		}
		duration := time.Since(startTime)

		mDuration.WithField("url", t.URL).Duration(duration)

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
