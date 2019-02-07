package statsd

import (
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common/testutil"
)

func TestCollector(t *testing.T) {
	testutil.BaseTest(t, New,
		func(_ []stats.SampleContainer, output string) string {
			return output
		})
}
