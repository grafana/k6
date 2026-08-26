package protocol_test

import (
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
)

func TestMetricMap(t *testing.T) {
	t.Parallel()

	t.Run("initializes metrics", func(t *testing.T) {
		t.Parallel()

		const foo = "foo_metric"

		mm := protocol.NewMetricMap(foo)
		fooMetric, hasFoo := mm.Map()[foo]
		if !hasFoo {
			t.Fatalf("foo should exist in the output map")
		}

		if fooMetric != 0 {
			t.Fatalf("foo should be zero")
		}
	})

	t.Run("increases counters", func(t *testing.T) {
		t.Parallel()

		const foo = "foo_metric"

		mm := protocol.NewMetricMap()
		if current := mm.Map(); current[foo] != 0 {
			t.Fatalf("uninitialized foo should be zero")
		}

		mm.Inc(foo)
		if updated := mm.Map(); updated[foo] != 1 {
			t.Fatalf("metric was not incremented")
		}
	})
}
