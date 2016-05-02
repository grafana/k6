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
	r    *Runner
	ctx  context.Context
	ch   chan runner.Result
	mods map[string]Module
}

type Module map[string]Member

type Member interface{}

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
		w := v8worker.New(vu.Recv, vu.RecvSync)

		w.Load("internal:constants", fmt.Sprintf(`var __id = %d;`, id))

		for _, f := range r.stdlib {
			if err := w.Load(f.Filename, f.Source); err != nil {
				log.WithError(err).WithField("file", f.Filename).Error("Couldn't load lib")
			}
		}

		if err := vu.RegisterModules(w); err != nil {
			log.WithError(err).Error("Couldn't register bridged functions")
			return
		}

		src := fmt.Sprintf(`
		function __run__() {
			try {
		%s
			} catch (e) {
				console.error(e);
			}
		}
		`, r.Source)
		if err := w.Load(r.Filename, src); err != nil {
			log.WithError(err).Error("Couldn't load JS")
			return
		}

		for {
			// log.Info("-> run")
			w.SendSync("run")
			// log.Info("<- run")

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return ch
}
