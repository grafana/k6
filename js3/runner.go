package js3

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"math"
	"time"
)

type Runner struct {
	Filename string
	Source   string
	Client   *fasthttp.Client
}

func New(filename, src string) *Runner {
	return &Runner{
		Filename: filename,
		Source:   src,
		Client: &fasthttp.Client{
			MaxConnsPerHost: math.MaxInt32,
		},
	}
}

func (r *Runner) RunVU(ctx context.Context, t speedboat.Test, id int) {
	mDuration := sampler.Stats("request.duration")
	mErrors := sampler.Counter("request.error")

	vm := duktape.New()

	vm.PushGlobalGoFunction("sleep", func(vm *duktape.Context) int {
		t := vm.ToInt(0)
		time.Sleep(time.Duration(t) * time.Second)
		return 0
	})
	vm.PushGlobalGoFunction("get", func(vm *duktape.Context) int {
		url := vm.ToString(0)

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		req.SetRequestURI(url)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		startTime := time.Now()
		err := r.Client.Do(req, res)
		duration := time.Since(startTime)

		mDuration.WithFields(sampler.Fields{
			"url":    url,
			"method": "GET",
			"status": res.StatusCode(),
		}).Duration(duration)

		if err != nil {
			log.WithError(err).Error("Request error")
			mErrors.WithFields(sampler.Fields{
				"url":    url,
				"method": "GET",
				"error":  err,
			}).Int(1)
		}

		return 0
	})

	vm.PushString(r.Filename)
	if err := vm.PcompileStringFilename(0, r.Source); err != nil {
		log.WithError(err).Error("Couldn't compile script")
		return
	}

	for {
		vm.DupTop()
		if vm.Pcall(0) != duktape.ErrNone {
			err := vm.SafeToString(-1)
			log.WithField("error", err).Error("Script error")
		}
		vm.Pop()
	}
}
