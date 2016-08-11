package js

import (
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func BenchmarkCallGoFunction(b *testing.B) {
	i := 0
	vm := otto.New()
	vm.Set("fn", func(call otto.FunctionCall) otto.Value {
		i += 1
		return otto.UndefinedValue()
	})
	script, err := vm.Compile("script", `fn();`)
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := vm.Run(script); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallGoFunctionReturn(b *testing.B) {
	i := 0
	vm := otto.New()
	vm.Set("fn", func(call otto.FunctionCall) otto.Value {
		i += 1
		v, err := otto.ToValue(i)
		if err != nil {
			panic(err)
		}
		return v
	})
	script, err := vm.Compile("script", `fn();`)
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := vm.Run(script); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallJSFunction(b *testing.B) {
	vm := otto.New()

	_, err := vm.Eval(`var i = 0; function fn() { i++; };`)
	assert.Nil(b, err)

	script, err := vm.Compile("script", `fn();`)
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := vm.Run(script); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallJSFunctionExplicitUndefined(b *testing.B) {
	vm := otto.New()

	_, err := vm.Eval(`var i = 0; function fn() { i++; return undefined; };`)
	assert.Nil(b, err)

	script, err := vm.Compile("script", `fn();`)
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := vm.Run(script); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCallJSFunctionReturn(b *testing.B) {
	vm := otto.New()

	_, err := vm.Eval(`var i = 0; function fn() { i++; return i; };`)
	assert.Nil(b, err)

	script, err := vm.Compile("script", `fn();`)
	assert.Nil(b, err)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := vm.Run(script); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunScriptParallelMultipleVMs(b *testing.B) {
	for n := 0; n < b.N; n++ {
		b.StopTimer()

		start := sync.WaitGroup{}
		start.Add(1)

		end := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			end.Add(1)
			go func() {
				defer end.Done()

				vm := otto.New()

				var i int
				vm.Set("fn", func(call otto.FunctionCall) otto.Value {
					i += 1
					v, err := call.Otto.ToValue(i)
					if err != nil {
						panic(err)
					}
					return v
				})

				script, err := vm.Compile("inline", `fn();`)
				if err != nil {
					b.Fatal(err)
				}
				start.Wait()
				if _, err := vm.Run(script); err != nil {
					b.Fatal(err)
				}
			}()
		}

		b.StartTimer()
		start.Done()
		end.Wait()
	}
}

func BenchmarkRunScriptParallelClonedVMs(b *testing.B) {
	vm := otto.New()

	var i int
	vm.Set("fn", func(call otto.FunctionCall) otto.Value {
		i += 1
		v, err := call.Otto.ToValue(i)
		if err != nil {
			panic(err)
		}
		return v
	})

	script, err := vm.Compile("inline", `fn();`)
	if err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		b.StopTimer()

		start := sync.WaitGroup{}
		start.Add(1)

		end := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			end.Add(1)
			go func() {
				defer end.Done()

				myVM := vm.Copy()

				start.Wait()
				if _, err := myVM.Run(script); err != nil {
					b.Fatal(err)
				}
			}()
		}

		b.StartTimer()
		start.Done()
		end.Wait()
	}
}

func BenchmarkRunScriptParallelClonedVMsNoSharedState(b *testing.B) {
	vm := otto.New()

	script, err := vm.Compile("inline", `fn();`)
	if err != nil {
		b.Fatal(err)
	}

	for n := 0; n < b.N; n++ {
		b.StopTimer()

		start := sync.WaitGroup{}
		start.Add(1)

		end := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			end.Add(1)
			go func() {
				defer end.Done()

				myVM := vm.Copy()

				var i int
				myVM.Set("fn", func(call otto.FunctionCall) otto.Value {
					i += 1
					v, err := call.Otto.ToValue(i)
					if err != nil {
						panic(err)
					}
					return v
				})

				start.Wait()
				if _, err := myVM.Run(script); err != nil {
					b.Fatal(err)
				}
			}()
		}

		b.StartTimer()
		start.Done()
		end.Wait()
	}
}
