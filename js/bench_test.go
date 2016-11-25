package js

import (
	"github.com/robertkrimen/otto"
	"testing"
)

func BenchmarkOttoRun(b *testing.B) {
	vm := otto.New()
	src := `1 + 1`

	b.Run("string", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := vm.Run(src)
			if err != nil {
				b.Error(err)
				return
			}
		}
	})

	b.Run("*Script", func(b *testing.B) {
		script, err := vm.Compile("__snippet__", src)
		if err != nil {
			b.Error(err)
			return
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := vm.Run(script)
			if err != nil {
				b.Error(err)
				return
			}
		}
	})
}
