package v8js

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/runner/v8js/bridge"
	"github.com/ry/v8worker"
)

type jsCallEnvelope struct {
	Mod  string        `json:"m"`
	Fn   string        `json:"f"`
	Args []interface{} `json:"a"`
}

func (vu *VUContext) bridgeAPI(w *v8worker.Worker) error {
	base := api.New()
	vu.api = make(map[string]bridge.Module)

	for modname, mod := range base {
		bmod := bridge.BridgeModule(modname, mod)
		vu.api[modname] = bmod

		js := bmod.JS()
		println(js)
		if err := w.Load("internal:api:"+modname, js); err != nil {
			log.WithError(err).WithField("mod", modname).Error("Couldn't bridge module")
			return err
		}
	}

	return nil
}

func (vu *VUContext) invoke(call jsCallEnvelope) error {
	return nil
}

func (vu *VUContext) Recv(raw string) {
	call := jsCallEnvelope{}
	if err := json.Unmarshal([]byte(raw), &call); err != nil {
		log.WithError(err).Error("Malformed host call")
		return
	}
	log.WithFields(log.Fields{
		"mod":  call.Mod,
		"fn":   call.Fn,
		"args": call.Args,
	}).Debug("Async call")

	if err := vu.invoke(call); err != nil {
		log.WithError(err).Error("Couldn't invoke")
	}
}

func (vu *VUContext) RecvSync(raw string) string {
	call := jsCallEnvelope{}
	if err := json.Unmarshal([]byte(raw), &call); err != nil {
		return jsThrow(fmt.Sprintf("malformed host call: %s", err))
	}
	log.WithFields(log.Fields{
		"mod":  call.Mod,
		"fn":   call.Fn,
		"args": call.Args,
	}).Debug("Sync call")

	if err := vu.invoke(call); err != nil {
		return jsThrow(err.Error())
	}
	return ""
}
