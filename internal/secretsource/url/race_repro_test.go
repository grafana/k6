package url

// Minimal reproduction for a data race detected in Go 1.27-devel (gotip) CI.
//
// The race is in runtime.moduleTypelinks() — a Go runtime-internal type cache
// map — triggered when encoding/json calls reflect.PointerTo() concurrently
// from multiple goroutines.
//
// CI stack trace (gotip @ 9777cec, linux/amd64):
//
//	Goroutine 85: mapaccess2_fast64 → moduleTypelinks → reflect.PointerTo → json.newTypeEncoder
//	              → json.Encoder.Encode  (url_test.go:1207)
//	Goroutine 86: mapassign_fast64  → moduleTypelinks → reflect.PointerTo → json.newTypeEncoder
//	              → json.Encoder.Encode  (url_test.go:1167)
//
// Reproduce with gotip @ 9777cec:
//
//	gotip test -race -count=100 -failfast -v ./internal/secretsource/url/ -run TestConcurrentJSONEncode_RaceRepro

import (
	"encoding/json"
	"io"
	"sync"
	"testing"
)

// TestConcurrentJSONEncode_RaceRepro hammers json.NewEncoder().Encode() from
// many goroutines in a tight loop. Each iteration creates a fresh encoder
// writing to io.Discard and encodes a map value. On gotip builds that carry
// the moduleTypelinks race, the race detector should flag this quickly.
func TestConcurrentJSONEncode_RaceRepro(t *testing.T) {
	t.Parallel()

	const (
		workers    = 64
		iterations = 2000
	)

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				resp := map[string]interface{}{
					"code":    "not_found",
					"message": "Secret not found",
					"nested": map[string]string{
						"key": "value",
					},
				}
				_ = json.NewEncoder(io.Discard).Encode(resp)
			}
		}()
	}

	wg.Wait()
}
