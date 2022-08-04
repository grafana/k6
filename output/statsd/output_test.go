package statsd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func getOutput(
	logger logrus.FieldLogger, addr, namespace null.String, bufferSize null.Int, pushInterval types.NullDuration,
) (*Output, error) {
	return newOutput(
		output.Params{
			Logger: logger,
			JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"addr": "%s",
			"namespace": "%s",
			"bufferSize": %d,
			"pushInterval": "%s"
		}`, addr.String, namespace.String, bufferSize.Int64, pushInterval.Duration.String())),
		})
}

func TestStatsdOutput(t *testing.T) {
	t.Parallel()
	baseTest(t, getOutput,
		func(t *testing.T, _ []metrics.SampleContainer, expectedOutput, output string) {
			assert.Equal(t, expectedOutput, output)
		})
}

func TestStatsdEnabledTags(t *testing.T) {
	t.Parallel()
	tagMap := metrics.EnabledTags{"tag1": true, "tag2": true}

	baseTest(t, func(
		logger logrus.FieldLogger, addr, namespace null.String, bufferSize null.Int, pushInterval types.NullDuration,
	) (*Output, error) {
		return newOutput(
			output.Params{
				Logger: logger,
				JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"addr": "%s",
			"namespace": "%s",
			"bufferSize": %d,
			"pushInterval": "%s",
			"tagBlocklist": ["tag1", "tag2"],
			"enableTags": true
		}`, addr.String, namespace.String, bufferSize.Int64, pushInterval.Duration.String())),
			})
	}, func(t *testing.T, containers []metrics.SampleContainer, expectedOutput, output string) {
		outputLines := strings.Split(output, "\n")
		expectedOutputLines := strings.Split(expectedOutput, "\n")
		var lines int

		for i, container := range containers {
			for j, sample := range container.GetSamples() {
				lines++
				var (
					expectedTagList    = processTags(tagMap, sample.GetTags().CloneTags())
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
		require.Equal(t, lines, len(outputLines))
	})
}

func TestInitWithoutAddressErrors(t *testing.T) {
	t.Parallel()
	c := &Output{
		config: config{},
		logger: testutils.NewLogger(t),
	}
	err := c.Start()
	require.Error(t, err)
}

func TestInitWithBogusAddressErrors(t *testing.T) {
	t.Parallel()
	c := &Output{
		config: config{
			Addr: null.StringFrom("localhost:90000"),
		},
		logger: testutils.NewLogger(t),
	}
	err := c.Start()
	require.Error(t, err)
}

func TestLinkReturnAddress(t *testing.T) {
	t.Parallel()
	bogusValue := "bogus value"
	c := &Output{
		config: config{
			Addr: null.StringFrom(bogusValue),
		},
	}
	require.Equal(t, fmt.Sprintf("statsd (%s)", bogusValue), c.Description())
}
