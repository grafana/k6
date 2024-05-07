package json

import (
	"bytes"
	"path"
	"testing"

	"github.com/liuxd6825/k6server/lib/fsext"
	"github.com/liuxd6825/k6server/lib/testutils"
	"github.com/liuxd6825/k6server/output"
	"github.com/stretchr/testify/require"
)

func BenchmarkFlushMetrics(b *testing.B) {
	stdout := new(bytes.Buffer)
	dir := b.TempDir()
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(b),
		StdOut:         stdout,
		FS:             fsext.NewOsFs(),
		ConfigArgument: path.Join(dir, "test.gz"),
	})
	require.NoError(b, err)

	samples, _ := generateTestMetricSamples(b)
	size := 10000
	for len(samples) < size {
		more, _ := generateTestMetricSamples(b)
		samples = append(samples, more...)
	}
	samples = samples[:size]
	o, _ := out.(*Output)
	require.NoError(b, o.Start())
	o.periodicFlusher.Stop()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		o.AddMetricSamples(samples)
		o.flushMetrics()
	}
}
