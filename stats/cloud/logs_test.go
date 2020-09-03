/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package cloud

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/testutils"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMsgParsing(t *testing.T) {
	m := `{
  "streams": [
    {
      "stream": {
      	"key1": "value1",
	   	"key2": "value2"
      },
      "values": [
        [
      	"1598282752000000000",
		"something to log"
        ]
      ]
    }
  ],
  "dropped_entries": [
    {
      "labels": {
      	"key3": "value1",
	   	"key4": "value2"
      },
      "timestamp": "1598282752000000000"
    }
  ]
}
`
	expectMsg := msg{
		Streams: []msgStreams{
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2"},
				Values: [][2]string{{"1598282752000000000", "something to log"}},
			},
		},
		DroppedEntries: []msgDroppedEntries{
			{
				Labels:    map[string]string{"key3": "value1", "key4": "value2"},
				Timestamp: "1598282752000000000",
			},
		},
	}
	var message msg
	require.NoError(t, easyjson.Unmarshal([]byte(m), &message))
	require.Equal(t, expectMsg, message)
}

func TestMSGLog(t *testing.T) {
	expectMsg := msg{
		Streams: []msgStreams{
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2"},
				Values: [][2]string{{"1598282752000000000", "something to log"}},
			},
			{
				Stream: map[string]string{"key1": "value1", "key2": "value2", "level": "warn"},
				Values: [][2]string{{"1598282752000000000", "something else log"}},
			},
		},
		DroppedEntries: []msgDroppedEntries{
			{
				Labels:    map[string]string{"key3": "value1", "key4": "value2", "level": "panic"},
				Timestamp: "1598282752000000000",
			},
		},
	}

	logger := logrus.New()
	logger.Out = ioutil.Discard
	hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
	logger.AddHook(hook)
	expectMsg.Log(logger)
	logLines := hook.Drain()
	assert.Equal(t, 4, len(logLines))
	expectTime := time.Unix(0, 1598282752000000000)
	for i, entry := range logLines {
		var expectedMsg string
		switch i {
		case 0:
			expectedMsg = "something to log"
		case 1:
			expectedMsg = "last message had unknown level "
		case 2:
			expectedMsg = "something else log"
		case 3:
			expectedMsg = "dropped"
		}
		require.Equal(t, expectedMsg, entry.Message)
		require.Equal(t, expectTime, entry.Time)
	}
}
