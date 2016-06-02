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
	defer js.Destroy()

	if err := putScript(js, r.Filename, r.Source); err != nil {
		log.WithError(err).Error("Couldn't compile script")
		return
	}

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
	}

	js.PushGlobalObject()
	js.PushString(scriptProp)
	for {
		js.DupTop()
		if js.PcallProp(-3, 0) != duktape.ErrNone {
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
