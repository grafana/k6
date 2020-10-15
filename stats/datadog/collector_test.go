/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package datadog

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
	"github.com/loadimpact/k6/stats/statsd/common/testutil"
)

func TestCollector(t *testing.T) {
	tagMap := stats.TagSet{"tag1": true, "tag2": true}
	handler := tagHandler(tagMap)
	testutil.BaseTest(t, func(
		logger logrus.FieldLogger, addr, namespace null.String, bufferSize null.Int,
		pushInterval types.NullDuration) (*common.Collector, error) {
		return New(logger, Config{
			Addr:         addr,
			Namespace:    namespace,
			BufferSize:   bufferSize,
			PushInterval: pushInterval,
			TagBlacklist: tagMap,
		})
	}, func(t *testing.T, containers []stats.SampleContainer, expectedOutput, output string) {
		outputLines := strings.Split(output, "\n")
		expectedOutputLines := strings.Split(expectedOutput, "\n")
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
