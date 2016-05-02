package v8js

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/ry/v8worker"
	"reflect"
)

func (vu *VUContext) RegisterModules(w *v8worker.Worker) error {
	vu.mods = map[string]Module{
		"global": Module{
			"sleep": vu.Sleep,
		},
		"console": Module{
			"log":   vu.ConsoleLog,
			"warn":  vu.ConsoleWarn,
			"error": vu.ConsoleError,
		},
		"http": Module{
			"get": vu.HTTPGet,
		},
	}

	for modname, mod := range vu.mods {
		jsMod := fmt.Sprintf(`
		speedboat._modules["%s"] = {};
		`, modname)
		for name, mem := range mod {
			t := reflect.TypeOf(mem)

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

			jsFn += `
				return speedboat._invoke('` + modname + `', '` + name + `', args);
			}`
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
}

func (vu *VUContext) RecvSync(raw string) string {
	call := struct {
		Mod  string        `json:"m"`
		Fn   string        `json:"f"`
		Args []interface{} `json:"a"`
	}{}
	if err := json.Unmarshal([]byte(raw), &call); err != nil {
		return jsThrow(fmt.Sprintf("malformed host call: %s", err))
	}
	log.WithFields(log.Fields{
		"mod":  call.Mod,
		"fn":   call.Fn,
		"args": call.Args,
	}).Debug("Sync call")

	mod, ok := vu.mods[call.Mod]
	if !ok {
		return jsThrow(fmt.Sprintf("unknown module '%s'", call.Mod))
	}

	fn, ok := mod[call.Fn]
	if !ok {
		return jsThrow(fmt.Sprintf("unrecognized function call: '%s'.'%s'", call.Mod, call.Fn))
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
	fnV := reflect.ValueOf(fn)
	log.WithField("T", fnV.Type().String()).Debug("Function")
	reflect.ValueOf(fn).Call(args)

	return ""
}
