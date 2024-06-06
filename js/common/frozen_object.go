package common

import (
	"fmt"

	"github.com/grafana/sobek"
)

// FreezeObject replicates the JavaScript Object.freeze function.
func FreezeObject(rt *sobek.Runtime, obj sobek.Value) error {
	global := rt.GlobalObject().Get("Object").ToObject(rt)
	freeze, ok := sobek.AssertFunction(global.Get("freeze"))
	if !ok {
		panic("failed to get the Object.freeze function from the runtime")
	}
	isFrozen, ok := sobek.AssertFunction(global.Get("isFrozen"))
	if !ok {
		panic("failed to get the Object.isFrozen function from the runtime")
	}
	fobj := &freezing{
		global:   global,
		rt:       rt,
		freeze:   freeze,
		isFrozen: isFrozen,
	}
	return fobj.deepFreeze(obj)
}

type freezing struct {
	rt       *sobek.Runtime
	global   sobek.Value
	freeze   sobek.Callable
	isFrozen sobek.Callable
}

func (f *freezing) deepFreeze(val sobek.Value) error {
	if val != nil && sobek.IsNull(val) {
		return nil
	}

	_, err := f.freeze(sobek.Undefined(), val)
	if err != nil {
		return fmt.Errorf("object freeze failed: %w", err)
	}

	o := val.ToObject(f.rt)
	if o == nil {
		return nil
	}

	for _, key := range o.Keys() {
		prop := o.Get(key)
		if prop == nil {
			continue
		}
		frozen, err := f.isFrozen(sobek.Undefined(), prop)
		if err != nil {
			return err
		}
		if frozen.ToBoolean() { // prevent cycles
			continue
		}
		if err = f.deepFreeze(prop); err != nil {
			return fmt.Errorf("deep freezing the property %s failed: %w", key, err)
		}
	}

	return nil
}
