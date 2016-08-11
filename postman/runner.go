package postman

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"time"
)

var (
	mRequests = stats.Stat{Name: "requests", Type: stats.HistogramType, Intent: stats.TimeIntent}
	mErrors   = stats.Stat{Name: "errors", Type: stats.CounterType}
)

const SETUP_SRC = `
// Scripts populate this with test resultks; keys are strings, values are bools,
// or just something truthy/falsy, because this is javascript.
var tests = {};
`

type ErrorWithLineNumber struct {
	Wrapped error
	Line    int
}

func (e ErrorWithLineNumber) Error() string {
	return fmt.Sprintf("%s (line %d)", e.Wrapped.Error(), e.Line)
}

type Runner struct {
	VM         *otto.Otto
	Collection Collection
	Endpoints  []Endpoint
}

type VU struct {
	Runner    *Runner
	VM        *otto.Otto
	Client    http.Client
	Collector *stats.Collector
}

func New(source []byte) (*Runner, error) {
	var collection Collection
	if err := json.Unmarshal(source, &collection); err != nil {
		switch e := err.(type) {
		case *json.SyntaxError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		case *json.UnmarshalTypeError:
			src := string(source)
			line := strings.Count(src[:e.Offset], "\n") + 1
			return nil, ErrorWithLineNumber{Wrapped: e, Line: line}
		}
		return nil, err
	}

	vm := otto.New()
	if _, err := vm.Eval(SETUP_SRC); err != nil {
		return nil, err
	}

	eps, err := MakeEndpoints(collection, vm)
	if err != nil {
		return nil, err
	}

	return &Runner{
		Collection: collection,
		Endpoints:  eps,
		VM:         vm,
	}, nil
}

func (r *Runner) NewVU() (lib.VU, error) {
	return &VU{
		Runner: r,
		VM:     r.VM.Copy(),
		Client: http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: math.MaxInt32,
			},
		},
		Collector: stats.NewCollector(),
	}, nil
}

func (u *VU) Reconfigure(id int64) error {
	return nil
}

func (u *VU) RunOnce(ctx context.Context) error {
	for _, ep := range u.Runner.Endpoints {
		req := ep.Request()

		startTime := time.Now()
		res, err := u.Client.Do(&req)
		duration := time.Since(startTime)

		status := 0
		if err == nil {
			status = res.StatusCode
			io.Copy(ioutil.Discard, res.Body)
			res.Body.Close()
		}

		tags := stats.Tags{"method": ep.Method, "url": ep.URLString, "status": status}
		u.Collector.Add(stats.Sample{
			Stat:   &mRequests,
			Tags:   tags,
			Values: stats.Values{"duration": float64(duration)},
		})

		if err != nil {
			log.WithError(err).Error("Request error")
			u.Collector.Add(stats.Sample{
				Stat:   &mErrors,
				Tags:   tags,
				Values: stats.Value(1),
			})
			return err
		}

		for _, script := range ep.Tests {
			if _, err := u.VM.Run(script); err != nil {
				return err
			}
		}
	}

	return nil
}
