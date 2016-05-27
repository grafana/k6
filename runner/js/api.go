package js

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"gopkg.in/olebedev/go-duktape.v2"
)

type apiFunc func(r *Runner, c *duktape.Context, ch chan<- runner.Result) int

func apiHTTPDo(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	method := argString(c, 0)
	if method == "" {
		ch <- runner.Result{Error: errors.New("Missing method in http call")}
		return 0
	}

	url := argString(c, 1)
	if url == "" {
		ch <- runner.Result{Error: errors.New("Missing URL in http call")}
		return 0
	}

	body := ""
	switch c.GetType(2) {
	case duktape.TypeNone, duktape.TypeNull, duktape.TypeUndefined:
	case duktape.TypeString, duktape.TypeNumber, duktape.TypeBoolean:
		body = c.ToString(2)
	case duktape.TypeObject:
		body = c.JsonEncode(2)
	default:
		ch <- runner.Result{Error: errors.New("Unknown type for request body")}
		return 0
	}

	args := httpArgs{}
	if err := argJSON(c, 3, &args); err != nil {
		ch <- runner.Result{Error: errors.New("Invalid arguments to http call")}
		return 0
	}

	res, duration, err := httpDo(r.Client, method, url, body, args)
	if !args.Quiet {
		ch <- runner.Result{Error: err, Time: duration}
	}

	pushInstance(c, res, "HTTPResponse")

	return 1
}

func apiHTTPSetMaxConnectionsPerHost(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	num := int(argNumber(c, 0))
	if num < 1 {
		ch <- runner.Result{Error: errors.New("Max connections per host must be at least 1")}
		return 0
	}
	r.Client.MaxConnsPerHost = num
	return 0
}

func apiLogType(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	kind := argString(c, 0)
	text := argString(c, 1)
	extra := map[string]interface{}{}
	if err := argJSON(c, 2, &extra); err != nil {
		ch <- runner.Result{Error: errors.New("Log context is not an object")}
		return 0
	}

	l := log.WithFields(log.Fields(extra))
	switch kind {
	case "debug":
		l.Debug(text)
	case "info":
		l.Info(text)
	case "warn":
		l.Warn(text)
	case "error":
		l.Error(text)
	}

	return 0
}

func apiTestAbort(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	ch <- runner.Result{Abort: true}
	return 0
}
