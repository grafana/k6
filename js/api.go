package js

import (
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/js/http"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"time"
)

type APIFunc func(js *duktape.Context, ctx context.Context) int

func apiSleep(js *duktape.Context, ctx context.Context) int {
	time.Sleep(time.Duration(argNumber(js, 0) * float64(time.Second)))
	return 0
}

func apiHTTPDo(js *duktape.Context, ctx context.Context) int {
	method := argString(js, 0)
	if method == "" {
		log.Error("Missing method in http call")
		return 0
	}

	url := argString(js, 1)
	if url == "" {
		log.Error("Missing URL in http call")
		return 0
	}

	body := ""
	switch js.GetType(2) {
	case duktape.TypeNone, duktape.TypeNull, duktape.TypeUndefined:
	case duktape.TypeString, duktape.TypeNumber, duktape.TypeBoolean:
		body = js.ToString(2)
	case duktape.TypeObject:
		body = js.JsonEncode(2)
	default:
		log.Error("Unknown type for request body")
		return 0
	}

	args := http.Args{}
	if err := argJSON(js, 3, &args); err != nil {
		log.Error("Invalid arguments to http call")
		return 0
	}

	res, err := http.Do(ctx, method, url, body, args)
	if err != nil {
		log.WithError(err).Error("Request error")
	}

	pushObject(js, res, "HTTPResponse")

	return 1
}
