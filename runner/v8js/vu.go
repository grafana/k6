package v8js

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/runner"
	"github.com/ry/v8worker"
	"reflect"
	"strings"
)

type jsCallEnvelope struct {
	Mod  string        `json:"m"`
	Fn   string        `json:"f"`
	Args []interface{} `json:"a"`
}

// Aaaaaa, this is awful, it needs restructuring BADLY x_x
func (vu *VUContext) BridgeAPI(w *v8worker.Worker) error {
	for modname, mod := range vu.api {
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
				switch aT.Kind() {
				case reflect.Struct:
					types := make([]string, 0, aT.NumField())
					for i := 0; i < aT.NumField(); i++ {
						field := aT.Field(i)
						if field.Anonymous {
							continue
						}
						key := field.Tag.Get("json") // Does not handle comma params yet!
						if key == "" {
							key = field.Name
						}
						val := aT.Kind().String()
						types = append(types, fmt.Sprintf(`"%s":"%s"`, key, val))
					}
					jsFn += fmt.Sprintf(`args.push(speedboat._require.struct({%s}, arguments[%d]));`, strings.Join(types, ","), i)
				default:
					jsFn += fmt.Sprintf("args.push(speedboat._require.%s(arguments[%d]));", aT.Kind().String(), i)
				}
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
			}`, modname, name, false)
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
	mod, ok := vu.api[call.Mod]
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

	fn := reflect.ValueOf(mem)
	fnT := fn.Type()

	for i := 0; i < fnT.NumIn(); i++ {
		argT := fnT.In(i)
		switch argT.Kind() {
		case reflect.Struct:
			mapv, ok := args[i].Interface().(map[string]interface{})
			if !ok {
				return errors.New("argument is not a dictionary")
			}

			v := reflect.New(argT)
			for i := 0; i < argT.NumField(); i++ {
				f := argT.Field(i)

				key := f.Tag.Get("json")
				if key == "" {
					key = f.Name
				}
				val, ok := mapv[key]
				if ok {
					v.Elem().Field(i).Set(reflect.ValueOf(val))
				}
			}

			args[i] = v.Elem()
		default:
		}
	}

	defer func() {
		if err := recover(); err != nil {
			log.WithField("error", err).Error("Go call panicked")
		}
	}()
	ret := fn.Call(args)

	for _, val := range ret {
		switch v := val.Interface().(type) {
		case <-chan runner.Result:
		readLoop:
			for {
				select {
				case <-vu.ctx.Done():
					break readLoop
				case r, ok := <-v:
					if !ok {
						break readLoop
					}
					vu.ch <- r
				}
			}
		default:
		}
	}

	return nil
}
