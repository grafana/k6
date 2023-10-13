package js

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
)

func BenchmarkEmptyIteration(b *testing.B) {
	b.StopTimer()

	r, err := getSimpleRunner(b, "/script.js", `exports.default = function() { }`)
	require.NoError(b, err)

	ch := newDevNullSampleChannel()
	defer close(ch)

	initVU, err := r.NewVU(context.Background(), 1, 1, ch)
	require.NoError(b, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err = vu.RunOnce()
		require.NoError(b, err)
	}
}
