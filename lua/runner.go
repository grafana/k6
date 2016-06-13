package lua

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"github.com/loadimpact/speedboat/sampler"
	"github.com/valyala/fasthttp"
	"github.com/yuin/gopher-lua"
	"golang.org/x/net/context"
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
		Client:   &fasthttp.Client{},
	}
}

func (r *Runner) RunVU(ctx context.Context, t speedboat.Test, id int) {
	mDuration := sampler.Stats("request.duration")
	mErrors := sampler.Counter("request.error")

	L := lua.NewState()
	defer L.Close()

	fn, err := L.LoadString(r.Source)
	if err != nil {
		log.WithError(err).Error("Couldn't compile script")
		return
	}

	L.SetGlobal("sleep", L.NewFunction(func(L *lua.LState) int {
		time.Sleep(time.Duration(L.ToInt64(1)) * time.Second)
		return 0
	}))
	L.SetGlobal("get", L.NewFunction(func(L *lua.LState) int {
		url := L.ToString(1)

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
	}))

	for {
		L.Push(fn)
		if err := L.PCall(0, 0, nil); err != nil {
			log.WithError(err).Error("Script error")
		}
	}
}
