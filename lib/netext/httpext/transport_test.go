package httpext

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func BenchmarkMeasureAndEmitMetrics(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	samples := make(chan metrics.SampleContainer, 10)
	defer close(samples)
	go func() { // discard all metrics
		for range samples { //nolint:revive
		}
	}()
	logger := logrus.New()
	logger.Level = logrus.DebugLevel

	registry := metrics.NewRegistry()
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Samples:        samples,
		Logger:         logger,
	}
	t := transport{
		state:       state,
		ctx:         ctx,
		tagsAndMeta: &metrics.TagsAndMeta{Tags: registry.RootTagSet()},
	}

	unfRequest := &unfinishedRequest{
		tracer: &Tracer{},
		response: &http.Response{
			StatusCode: http.StatusOK,
		},
		request: &http.Request{
			URL: &url.URL{
				Host:   "example.com",
				Scheme: "https",
			},
		},
	}

	b.Run("no responseCallback", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			t.measureAndEmitMetrics(unfRequest)
		}
	})

	t.responseCallback = func(_ int) bool { return true }

	b.Run("responseCallback", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			t.measureAndEmitMetrics(unfRequest)
		}
	})
}
