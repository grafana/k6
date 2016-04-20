package v8js

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/ry/v8worker"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"strings"
	"time"
)

type Runner struct {
	Filename string
	Source   string
	Client   *fasthttp.Client
}

type VUContext struct {
	r   *Runner
	ctx context.Context
	ch  chan runner.Result
}

func New(filename, src string) *Runner {
	return &Runner{
		Filename: filename,
		Source:   src,
		Client: &fasthttp.Client{
			Dial:                fasthttp.Dial,
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt64,
		},
	}
}
func (r *Runner) Run(ctx context.Context, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		vu := VUContext{r: r, ctx: ctx, ch: ch}

		w := v8worker.New(func(msg string) {}, func(msg string) string {
			parts := strings.SplitN(msg, ";", 2)
			switch parts[0] {
			case "get":
				vu.HTTPGet(parts[1])
			case "sleep":
				vu.Sleep(parts[1])
			default:
				log.WithField("call", parts[0]).Fatal("Unknown JS call")
			}
			return ""
		})
		w.Load(r.Filename, fmt.Sprintf(`
		$recvSync(function(msg) {
			if(msg == 'run') {
				run()
			}
		})
		function get(url) {
			$sendSync('get;' + url)
		}
		function sleep(t) {
			$sendSync('sleep;' + t)
		}
		function run() {
			%s
		}
		`, r.Source))

		// vm := otto.New()
		// vm.Set("__id", id)
		// vm.Set("get", vu.HTTPGet)
		// vm.Set("sleep", vu.Sleep)

		// script, err := vm.Compile(r.Filename, r.Source)
		// if err != nil {
		// 	ch <- runner.Result{Error: err}
		// 	return
		// }

		for {
			// if _, err := vm.Run(script); err != nil {
			// 	ch <- runner.Result{Error: err}
			// }
			w.SendSync("run")

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch
}
