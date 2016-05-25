package js

import (
	"errors"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"gopkg.in/olebedev/go-duktape.v2"
	"time"
)

type apiFunc func(r *Runner, c *duktape.Context, ch chan<- runner.Result) int

type apiHTTPArgs struct {
	Report bool `json:"report"`
}

func apiHTTPGet(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	url := argString(c, 0)
	if url == "" {
		ch <- runner.Result{Error: errors.New("Missing URL in http.get()")}
		return 0
	}
	args := apiHTTPArgs{}
	if err := argJSON(c, 1, &args); err != nil {
		ch <- runner.Result{Error: errors.New("Invalid arguments to http.get()")}
		return 0
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(url)

	startTime := time.Now()
	err := r.Client.Do(req, res)
	duration := time.Since(startTime)

	if args.Report {
		ch <- runner.Result{Error: err, Time: duration}
	}

	return 0
}
