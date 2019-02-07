package datadog

import (
	"strings"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
	"github.com/loadimpact/k6/stats/statsd/common/testutil"
)

func TestCollector(t *testing.T) {
	var tagSet = lib.GetTagSet("tag1", "tag2")
	var handler = tagHandler(tagSet)
	testutil.BaseTest(t, func(config common.Config) (*common.Collector, error) {
		return New(NewConfig().Apply(Config{
			TagWhitelist: tagSet,
			Config:       config,
		}))
	}, func(containers []stats.SampleContainer, output string) string {
		var parts = strings.Split(output, "\n")
		for i, container := range containers {
			for j, sample := range container.GetSamples() {
				tagStrings := handler.processTags(sample.GetTags().CloneTags())
				if len(tagStrings) > 0 {
					parts[j*i+i] += "|#" + strings.Join(tagStrings, ",")
				}
			}
		}
		return strings.Join(parts, "\n")
	})
}
