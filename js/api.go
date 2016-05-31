package js

import (
	log "github.com/Sirupsen/logrus"
	"gopkg.in/olebedev/go-duktape.v2"
)

type apiFunc func(r *Runner, c *duktape.Context) int

func apiHTTPDo(r *Runner, c *duktape.Context) int {
	method := argString(c, 0)
	if method == "" {
		log.Error("Missing method in http call")
		return 0
	}

	url := argString(c, 1)
	if url == "" {
		log.Error("Missing URL in http call")
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
		log.Error("Unknown type for request body")
		return 0
	}

	args := httpArgs{}
	if err := argJSON(c, 3, &args); err != nil {
		log.Error("Invalid arguments to http call")
		return 0
	}

	res, duration, err := httpDo(r.Client, method, url, body, args)
	if err != nil {
		log.WithError(err).Error("Request error")
	}
	if !args.Quiet {
		r.mDuration.WithField("url", url).Duration(duration)
	}

	pushInstance(c, res, "HTTPResponse")

	return 1
}

func apiHTTPSetMaxConnectionsPerHost(r *Runner, c *duktape.Context) int {
	num := int(argNumber(c, 0))
	if num < 1 {
		log.Error("Max connections per host must be at least 1")
		return 0
	}
	r.Client.MaxConnsPerHost = num
	return 0
}

func apiLogType(r *Runner, c *duktape.Context) int {
	kind := argString(c, 0)
	text := argString(c, 1)
	extra := map[string]interface{}{}
	if err := argJSON(c, 2, &extra); err != nil {
		log.Error("Log context is not an object")
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

func apiTestAbort(r *Runner, c *duktape.Context) int {
	// TODO: Do this some better way.
	log.Fatal("Test aborted")
	return 0
}
