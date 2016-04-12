package js

import (
	"testing"
)

func BenchmarkRunIteration(b *testing.B) {
	r, err := New()
	if err != nil {
		b.Fatal(err)
	}
	err = r.Load("script.js", "")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for res := range r.RunIteration() {
			if err, ok := res.(error); ok {
				b.Error(err)
			}
		}
	}
}
