// Package faker contains Faker class implementation for sobek.
package faker

import (
	"math/rand"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/sobek"
	"lukechampine.com/frand"
)

// Constructor is a Faker class constructor.
func Constructor(call sobek.ConstructorCall, runtime *sobek.Runtime) *sobek.Object {
	seed := call.Argument(0).ToInteger()

	return runtime.NewDynamicObject(newFaker(seed, runtime))
}

// New calls Faker constructor and returns new Faker object.
func New(seed int64, runtime *sobek.Runtime) *sobek.Object {
	return Constructor(
		sobek.ConstructorCall{
			This:      runtime.NewObject(),
			Arguments: []sobek.Value{runtime.ToValue(seed)},
		},
		runtime,
	)
}

// faker represents JavaScript Faker class.
type faker struct {
	rand    *rand.Rand
	runtime *sobek.Runtime
}

// newFaker creates new Faker instance.
func newFaker(seed int64, runtime *sobek.Runtime) *faker {
	src := frand.NewSource()

	if seed != 0 {
		src.Seed(seed)
	}

	return &faker{rand: rand.New(src), runtime: runtime} //#nosec G404
}

// Delete implements sobek.DynamicObject.
func (f *faker) Delete(_ string) bool {
	return false
}

// Get implements sobek.DynamicObject.
func (f *faker) Get(key string) sobek.Value {
	if key == "call" {
		return f.runtime.ToValue(f.call)
	}

	category := newCategory(f, key)
	if category == nil {
		return sobek.Undefined()
	}

	return f.runtime.NewDynamicObject(category)
}

// Has implements sobek.DynamicObject.
func (f *faker) Has(_ string) bool {
	return false
}

// Keys implements sobek.DynamicObject.
func (f *faker) Keys() []string {
	return getCategoryNames()
}

// Set implements sobek.DynamicObject.
func (f *faker) Set(_ string, _ sobek.Value) bool {
	return false
}

// call invokes faker function by name.
// The faker function name is the first parameter, the rest of parameters passed to function.
func (f *faker) call(call sobek.FunctionCall) sobek.Value {
	function := call.Argument(0)

	if sobek.IsUndefined(function) {
		panic(f.runtime.NewTypeError(function))
	}

	info, found := lookupFunc(function.ToString().String())
	if !found {
		panic(f.runtime.NewTypeError(function))
	}

	call.Arguments = call.Arguments[1:]

	return f.invoke(info, call)
}

func (f *faker) toMapParams(info *gofakeit.Info, call sobek.FunctionCall) *gofakeit.MapParams {
	if len(info.Params) == 0 {
		return nil
	}

	params := gofakeit.NewMapParams()

	for idx, param := range info.Params {
		val := call.Argument(idx)
		if sobek.IsUndefined(val) {
			if len(param.Default) != 0 {
				params.Add(param.Field, param.Default)

				continue
			}

			if param.Optional {
				continue
			}

			panic(f.runtime.NewTypeError("missing parameter: %s", param.Field))
		}

		var arr []string

		if f.runtime.ExportTo(val, &arr) == nil {
			(*params)[param.Field] = arr
		} else {
			params.Add(param.Field, val.String())
		}
	}

	return params
}

func (f *faker) invoke(info *gofakeit.Info, call sobek.FunctionCall) sobek.Value {
	params := f.toMapParams(info, call)

	val, err := info.Generate(f.rand, params, info)
	if err != nil {
		panic(f.runtime.NewGoError(err))
	}

	return f.runtime.ToValue(val)
}

type category struct {
	faker *faker
	funcs map[string]*gofakeit.Info
}

func newCategory(faker *faker, name string) *category {
	funcs, ok := lookupCategory(name)
	if !ok {
		return nil
	}

	return &category{faker: faker, funcs: funcs}
}

// Delete implements sobek.DynamicObject.
func (c *category) Delete(_ string) bool {
	return false
}

// Get implements sobek.DynamicObject.
func (c *category) Get(key string) sobek.Value {
	info, ok := c.funcs[key]
	if !ok {
		return sobek.Undefined()
	}

	return c.faker.runtime.ToValue(func(call sobek.FunctionCall) sobek.Value {
		return c.faker.invoke(info, call)
	})
}

// Has implements sobek.DynamicObject.
func (c *category) Has(_ string) bool {
	return false
}

// Keys implements sobek.DynamicObject.
func (c *category) Keys() []string {
	return []string{}
}

// Set implements sobek.DynamicObject.
func (c *category) Set(_ string, _ sobek.Value) bool {
	return false
}
