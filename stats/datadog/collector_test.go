package datadog

import (
	"strings"
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
	"github.com/loadimpact/k6/stats/statsd/common/testutil"
	"github.com/stretchr/testify/require"
)

func TestCollector(t *testing.T) {
	var tagMap = stats.TagSet{"tag1": true, "tag2": true}
	var handler = tagHandler(tagMap)
	testutil.BaseTest(t, func(config common.Config) (*common.Collector, error) {
		return New(NewConfig().Apply(Config{
			TagBlacklist: tagMap,
			Config:       config,
		}))
	}, func(t *testing.T, containers []stats.SampleContainer, expectedOutput, output string) {
		var outputLines = strings.Split(output, "\n")
		var expectedOutputLines = strings.Split(expectedOutput, "\n")
		for i, container := range containers {
			for j, sample := range container.GetSamples() {
				var (
					expectedTagList    = handler.processTags(sample.GetTags().CloneTags())
					expectedOutputLine = expectedOutputLines[i*j+i]
					outputLine         = outputLines[i*j+i]
					outputWithoutTags  = outputLine
					outputTagList      = []string{}
					tagSplit           = strings.LastIndex(outputLine, "|#")
				)

				if tagSplit != -1 {
					outputWithoutTags = outputLine[:tagSplit]
					outputTagList = strings.Split(outputLine[tagSplit+len("|#"):], ",")
				}
				require.Equal(t, expectedOutputLine, outputWithoutTags)
				require.ElementsMatch(t, expectedTagList, outputTagList)
			}
		}
	})
}
