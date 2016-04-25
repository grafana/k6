package v8js

import (
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/ry/v8worker"
	"github.com/valyala/fasthttp"
	"golang.org/x/net/context"
	"math"
	"os"
	"strings"
	"time"
)

type libFile struct {
	Filename string
	Source   string
}

type Runner struct {
	Filename string
	Source   string
	Client   *fasthttp.Client

	stdlib []libFile
}

type VUContext struct {
	r   *Runner
	ctx context.Context
	ch  chan runner.Result
}

func New(filename, src string) *Runner {
	r := &Runner{
		Filename: filename,
		Source:   src,
		Client: &fasthttp.Client{
			Dial:                fasthttp.Dial,
			MaxIdleConnDuration: time.Duration(0),
			MaxConnsPerHost:     math.MaxInt64,
		},
	}

	// Load the standard library as a rice box; panic if any part of this fails
	// (The only possible cause is a programming/developer error, not user error)
	box := rice.MustFindBox("lib")
	box.Walk("/", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if !info.IsDir() {
			r.stdlib = append(r.stdlib, libFile{
				Filename: path,
				Source:   box.MustString(path),
			})
		}
		return nil
	})

	return r
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

		for _, f := range r.stdlib {
			w.Load(f.Filename, f.Source)
		}

		src := fmt.Sprintf("speedboat._internal.recv.run = function() {\n%s\n}", r.Source)
		w.Load(r.Filename, src)

		for {
			w.SendSync("{\"call\": \"run\"}")

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch
}
