package http

import (
	"github.com/valyala/fasthttp"
	"math"
	"time"
)

type context struct {
	client   *fasthttp.Client
	defaults RequestArgs
}

type RequestArgs struct {
	Follow    bool   `json:"follow"`
	Report    bool   `json:"report"`
	UserAgent string `json:"userAgent"`
}

func (args *RequestArgs) ApplyDefaults(def RequestArgs) {
	if !args.Follow && def.Follow {
		args.Follow = true
	}
	if !args.Report && def.Follow {
		args.Report = true
	}
	if args.UserAgent == "" {
		args.UserAgent = def.UserAgent
	}
}

func New() map[string]interface{} {
	ctx := &context{
		client: &fasthttp.Client{
			Dial:                fasthttp.Dial,
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt64,
		},
	}
	return map[string]interface{}{
		"get":                ctx.Get,
		"head":               ctx.Head,
		"post":               ctx.Post,
		"put":                ctx.Put,
		"delete":             ctx.Delete,
		"request":            ctx.Request,
		"setMaxConnsPerHost": ctx.SetMaxConnsPerHost,
	}
}
