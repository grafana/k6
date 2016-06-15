package js2

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/robertkrimen/otto"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
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

	vm := otto.New()
	script, err := vm.Compile(r.Filename, r.Source)
	if err != nil {
		log.WithError(err).Error("Couldn't compile script")
		return
	}

	vm.Set("sleep", func(call otto.FunctionCall) otto.Value {
		t, err := call.Argument(0).ToInteger()
		if err != nil {
			panic(err)
		}
		time.Sleep(time.Duration(t) * time.Second)
		return otto.UndefinedValue()
	})
	vm.Set("get", func(call otto.FunctionCall) otto.Value {
		url, err := call.Argument(0).ToString()
		if err != nil {
			panic(err)
		}

		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)

		req.SetRequestURI(url)

		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(res)

		startTime := time.Now()
		err = r.Client.Do(req, res)
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

		return otto.UndefinedValue()
	})

	for {
		if _, err := vm.Run(script); err != nil {
			log.WithError(err).Error("Script error")
		}
	}
}
