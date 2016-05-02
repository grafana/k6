package v8js

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/ry/v8worker"
	"reflect"
)

type jsCallEnvelope struct {
	Mod  string        `json:"m"`
	Fn   string        `json:"f"`
	Args []interface{} `json:"a"`
}

func (vu *VUContext) RegisterModules(w *v8worker.Worker) error {
	vu.mods = map[string]Module{
		"global": Module{
			"sleep": Member{Func: vu.Sleep},
		},
		"console": Module{
			"log":   Member{Func: vu.ConsoleLog, Async: true},
			"warn":  Member{Func: vu.ConsoleWarn, Async: true},
			"error": Member{Func: vu.ConsoleError, Async: true},
		},
		"http": Module{
			"get": Member{Func: vu.HTTPGet},
		},
	}

	for modname, mod := range vu.mods {
		jsMod := fmt.Sprintf(`
		speedboat._modules["%s"] = {};
		`, modname)
		for name, mem := range mod {
			t := reflect.TypeOf(mem.Func)

			if t.Kind() != reflect.Func {
				return errors.New("Not a function: " + modname + "." + name)
			}

			jsFn := fmt.Sprintf(`speedboat._modules["%s"]["%s"] = function() {
				var args = [];
			`, modname, name)

			numArgs := t.NumIn()
			if !t.IsVariadic() {
				jsFn += fmt.Sprintf(`
					if (arguments.length != %d) {
						throw new Error("wrong number of arguments");
					}
				`, t.NumIn())
			} else {
				numArgs--
			}

			for i := 0; i < numArgs; i++ {
				aT := t.In(i)
				jsFn += fmt.Sprintf("args.push(speedboat._require.%s(arguments[%d]));", aT.Kind().String(), i)
			}
			if t.IsVariadic() {
				varArg := t.In(numArgs)
				eT := varArg.Elem()
				jsFn += fmt.Sprintf(`
					for (var i = %d; i < arguments.length; i++) {
						args.push(speedboat._require.%s(arguments[i]));
					}
				`, numArgs, eT.Kind().String())
			}

			jsFn += fmt.Sprintf(`
				return speedboat._invoke('%s', '%s', args, %v);
			}`, modname, name, mem.Async)
			jsMod += "\n\n" + jsFn
		}

		if err := w.Load("module:"+modname, jsMod); err != nil {
			return err
		}
	}

	// Make functions in the "global" module global, preimport console
	makeGlobals := `
	for (key in speedboat._modules['global']) {
		eval(key + " = speedboat._modules['global']['" + key + "'];");
	}
	var console = speedboat._modules['console'];
	`
	if err := w.Load("internal:preload", makeGlobals); err != nil {
		return err
	}

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

func (vu *VUContext) invoke(call jsCallEnvelope) error {
	mod, ok := vu.mods[call.Mod]
	if !ok {
		return errors.New(fmt.Sprintf("unknown module '%s'", call.Mod))
	}

	mem, ok := mod[call.Fn]
	if !ok {
		return errors.New(fmt.Sprintf("unrecognized function call: '%s'.'%s'", call.Mod, call.Fn))
	}

	args := make([]reflect.Value, len(call.Args))
	for i, arg := range call.Args {
		args[i] = reflect.ValueOf(arg)
	}

	defer func() {
		if err := recover(); err != nil {
			log.WithField("error", err).Error("Go call panicked")
		}
	}()
	fn := reflect.ValueOf(mem.Func)
	log.WithField("T", fn.Type().String()).Debug("Function")
	fn.Call(args)

	return nil
}
