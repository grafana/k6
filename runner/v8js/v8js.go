package v8js

import (
	"encoding/json"
	"fmt"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/loadtest"
	"github.com/loadimpact/speedboat/runner"
	"github.com/ry/v8worker"
	"golang.org/x/net/context"
	"os"
)

type libFile struct {
	Filename string
	Source   string
}

type Runner struct {
	Filename string
	Source   string

	stdlib []libFile
}

type VUContext struct {
	r   *Runner
	ctx context.Context
	ch  chan runner.Result
	api map[string]map[string]interface{}
}

type workerData struct {
	ID        int64
	Test      loadtest.LoadTest
	Iteration int
}

func New(filename, src string) *Runner {
	r := &Runner{
		Filename: filename,
		Source:   src,
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

func (r *Runner) Run(ctx context.Context, t loadtest.LoadTest, id int64) <-chan runner.Result {
	ch := make(chan runner.Result)

	go func() {
		defer close(ch)

		vu := VUContext{r: r, ctx: ctx, ch: ch, api: api.New()}
		w := v8worker.New(vu.Recv, vu.RecvSync)

		for _, f := range r.stdlib {
			if err := w.Load(f.Filename, f.Source); err != nil {
				log.WithError(err).WithField("file", f.Filename).Error("Couldn't load lib")
			}
		}

		wdata := workerData{
			ID:   id,
			Test: t,
		}
		wjson, err := json.Marshal(wdata)
		if err != nil {
			log.WithError(err).Error("Couldn't encode worker data")
			return
		}
		w.Load("internal:constants", fmt.Sprintf(`speedboat._data = %s;`, wjson))

		if err := vu.BridgeAPI(w); err != nil {
			log.WithError(err).Error("Couldn't register bridged functions")
			return
		}

		src := fmt.Sprintf(`
		function __run__() {
			speedboat._data.Iteration++;
			try {
		%s
			} catch (e) {
				console.error("Script Error", '' + e);
			}
		}
		`, r.Source)
		if err := w.Load(r.Filename, src); err != nil {
			log.WithError(err).Error("Couldn't load JS")
			return
		}

		done := make(chan interface{})
		for {
			go func() {
				w.SendSync("run")
				done <- struct{}{}
			}()

			select {
			case <-done:
			case <-ctx.Done():
				w.TerminateExecution()
				return
			}
		}
	}()

	return ch
}
