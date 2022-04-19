package common

import (
	"fmt"

	"github.com/dop251/goja"
)

// FreezeObject replicates the JavaScript Object.freeze function.
func FreezeObject(rt *goja.Runtime, obj goja.Value) error {
	global := rt.GlobalObject().Get("Object").ToObject(rt)
	freeze, ok := goja.AssertFunction(global.Get("freeze"))
	if !ok {
		panic("failed to get the Object.freeze function from the runtime")
	}
	isFrozen, ok := goja.AssertFunction(global.Get("isFrozen"))
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
	rt       *goja.Runtime
	global   goja.Value
	freeze   goja.Callable
	isFrozen goja.Callable
}

func (f *freezing) deepFreeze(val goja.Value) error {
	if val != nil && goja.IsNull(val) {
		return nil
	}

	_, err := f.freeze(goja.Undefined(), val)
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
		frozen, err := f.isFrozen(goja.Undefined(), prop)
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
