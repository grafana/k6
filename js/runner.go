package js

import (
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat"
	"golang.org/x/net/context"
	"gopkg.in/olebedev/go-duktape.v2"
	"os"
)

type Runner struct {
	Filename string
	Source   string
}

func New(filename, src string) *Runner {
	return &Runner{
		Filename: filename,
		Source:   src,
	}
}

func (r *Runner) RunVU(ctx context.Context, t speedboat.Test, id int) {
	js := duktape.New()
	setupGlobalObject(js, t, id)
	bridgeAPI(js, contextForAPI(ctx))

	vendor, err := rice.FindBox("vendor")
	if err != nil {
		log.WithError(err).Error("Script vendor files missing; try `git submodule update --init`")
		return
	}
	vendorFiles := []string{"lodash/dist/lodash.min.js"}
	for _, filename := range vendorFiles {
		src, err := vendor.String(filename)
		if err != nil {
			log.WithError(err).Error("Couldn't read dependent script")
			return
		}
		if err := loadScript(js, filename, src); err != nil {
			log.WithError(err).Error("Couldn't load dependency")
			return
		}
	}

	lib, err := rice.FindBox("lib")
	if err != nil {
		log.WithError(err).Error("Script support files absent")
		return
	}
	if err := lib.Walk("/", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		src, err := lib.String(path)
		if err != nil {
			return err
		}
		return loadScript(js, path, src)
	}); err != nil {
		log.WithError(err).Error("Couldn't load support file")
		return
	}

	if err := pushScript(js, r.Filename, r.Source); err != nil {
		log.WithError(err).Error("Couldn't compile script")
		return
	}

	// js.PushGlobalObject()
	// js.PushString(scriptProp)
	for {
		js.DupTop()
		// if js.PcallProp(-3, 0) != duktape.ErrNone {
		if js.Pcall(0) != duktape.ErrNone {
			err := getJSError(js)
			log.WithFields(log.Fields{
				"file":  err.Filename,
				"line":  err.Line,
				"error": err.Message,
			}).Error("Script Error")
		}
		js.Pop()

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func bridgeAPI(js *duktape.Context, ctx context.Context) {
	api := map[string]map[string]APIFunc{
		"$http": map[string]APIFunc{
			"request":            apiHTTPRequest,
			"setMaxConnsPerHost": apiHTTPSetMaxConnsPerHost,
		},
		"$log": map[string]APIFunc{
			"log": apiLogLog,
		},
		"$test": map[string]APIFunc{
			"abort": apiTestAbort,
		},
		"$vu": map[string]APIFunc{
			"sleep": apiSleep,
		},
	}

	js.PushGlobalObject()
	defer js.Pop()

	for modname, mod := range api {
		js.PushObject()
		for fname, fn := range mod {
			fn := fn
			js.PushGoFunction(func(js *duktape.Context) int {
				return fn(js, ctx)
			})
			js.PutPropString(-2, fname)
		}
		js.PutPropString(-2, modname)
	}
}
