package js

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/loadimpact/speedboat/runner"
	"github.com/valyala/fasthttp"
	"gopkg.in/olebedev/go-duktape.v2"
	"net/url"
	"strings"
	"time"
)

type apiFunc func(r *Runner, c *duktape.Context, ch chan<- runner.Result) int

func apiHTTPDo(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	method := argString(c, 0)
	if method == "" {
		ch <- runner.Result{Error: errors.New("Missing method in http call")}
		return 0
	}

	u := argString(c, 1)
	if u == "" {
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

	args := struct {
		Quiet   bool              `json:"quiet"`
		Headers map[string]string `json:"headers"`
	}{}
	if err := argJSON(c, 3, &args); err != nil {
		ch <- runner.Result{Error: errors.New("Invalid arguments to http call")}
		return 0
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetMethod(method)

	if method == "GET" {
		if body != "" && body[0] == '{' {
			rawItems := map[string]interface{}{}
			if err := json.Unmarshal([]byte(body), &rawItems); err != nil {
				ch <- runner.Result{Error: err}
				return 0
			}
			parts := []string{}
			for key, value := range rawItems {
				value := url.QueryEscape(fmt.Sprint(value))
				parts = append(parts, fmt.Sprintf("%s=%s", key, value))
			}
			req.SetRequestURI(u + "?" + strings.Join(parts, "&"))
		} else {
			req.SetRequestURI(u)
		}
	} else {
		req.SetRequestURI(u)
		req.SetBodyString(body)
	}

	for key, value := range args.Headers {
		req.Header.Set(key, value)
	}

	startTime := time.Now()
	err := r.Client.Do(req, res)
	duration := time.Since(startTime)

	if !args.Quiet {
		ch <- runner.Result{Error: err, Time: duration}
	}

	index := c.PushObject()
	{
		c.PushNumber(float64(res.StatusCode()))
		c.PutPropString(-2, "status")

		c.PushString(string(res.Body()))
		c.PutPropString(-2, "body")

		c.PushObject()
		res.Header.VisitAll(func(key, value []byte) {
			c.PushString(string(value))
			c.PutPropString(-2, string(key))
		})
		c.PutPropString(-2, "headers")
	}

	c.PushGlobalObject()
	c.GetPropString(-1, "HTTPResponse")
	c.SetPrototype(index)
	c.Pop()

	return 1
}

func apiHTTPSetMaxConnectionsPerHost(r *Runner, c *duktape.Context, ch chan<- runner.Result) int {
	num := int(argNumber(c, 0))
	if num < 1 {
		ch <- runner.Result{Error: errors.New("Max connections per host must be at least 1")}
		return
	}
	r.Client.MaxConnsPerHost = num
	return 0
}
