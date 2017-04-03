package main

import (
	"context"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
)

func BenchmarkJSRunners(b *testing.B) {
	scripts := map[string]string{
		"Empty": `export default function() {}`,
		"HTTP":  `import http from "k6/http"; export default function() { http.get("http://localhost:8080/"); }`,
	}
	for name, script := range scripts {
		b.Run(name, func(b *testing.B) {
			for _, t := range []string{"js", "js2"} {
				b.Run(t, func(b *testing.B) {
					r, err := makeRunner(t, &lib.SourceData{
						Filename: "/script.js",
						Data:     []byte(script),
					}, afero.NewMemMapFs())
					if err != nil {
						b.Error(err)
						return
					}

					b.Run("Spawn", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							_, err := r.NewVU()
							if err != nil {
								b.Error(err)
								return
							}
						}
					})

					b.Run("Run", func(b *testing.B) {
						vu, err := r.NewVU()
						if err != nil {
							b.Error(err)
							return
						}
						b.ResetTimer()

						for i := 0; i < b.N; i++ {
							_, err := vu.RunOnce(context.Background())
							if err != nil {
								b.Error(err)
							}
						}
					})
				})
			}
		})
	}
}
