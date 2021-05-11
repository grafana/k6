/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package statsd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

// TODO delete this file as well when datadog output is dropped

func TestCollectorDatadog(t *testing.T) {
	t.Parallel()
	tagMap := stats.TagSet{"tag1": true, "tag2": true}
	baseTest(t, func(
		logger logrus.FieldLogger, addr, namespace null.String, bufferSize null.Int,
		pushInterval types.NullDuration) (*Output, error) {
		return NewDatadog(
			output.Params{
				Logger: logger,
				JSONConfig: json.RawMessage(fmt.Sprintf(`{
			"addr": "%s",
			"namespace": "%s",
			"bufferSize": %d,
			"pushInterval": "%s",
			"tagBlacklist": ["tag1", "tag2"]
		}`, addr.String, namespace.String, bufferSize.Int64, pushInterval.Duration.String())),
			})
	}, func(t *testing.T, containers []stats.SampleContainer, expectedOutput, output string) {
		outputLines := strings.Split(output, "\n")
		expectedOutputLines := strings.Split(expectedOutput, "\n")
		for i, container := range containers {
			for j, sample := range container.GetSamples() {
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
	})
}
