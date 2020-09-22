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

package log

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyslogFromConfigLine(t *testing.T) {
	t.Parallel()
	tests := [...]struct {
		line string
		err  bool
		res  lokiHook
	}{
		{
			line: "loki", // default settings
			res: lokiHook{
				ctx:           context.Background(),
				addr:          "http://127.0.0.1:3100/loki/api/v1/push",
				limit:         100,
				pushPeriod:    time.Second * 1,
				levels:        logrus.AllLevels,
				msgMaxSize:    1024 * 1024,
				droppedLabels: map[string]string{"level": "warning"},
			},
		},
		{
			line: "loki=somewhere:1233,label.something=else,label.foo=bar,limit=32,level=info,allowedLabels=[something],pushPeriod=5m32s,msgMaxSize=1231",
			res: lokiHook{
				ctx:           context.Background(),
				addr:          "somewhere:1233",
				limit:         32,
				pushPeriod:    time.Minute*5 + time.Second*32,
				levels:        logrus.AllLevels[:5],
				labels:        [][2]string{{"something", "else"}, {"foo", "bar"}},
				msgMaxSize:    1231,
				allowedLabels: []string{"something"},
				droppedLabels: map[string]string{"something": "else"},
			},
		},
		{
			line: "lokino",
			err:  true,
		},
		{
			line: "loki=something,limit=word",
			err:  true,
		},
		{
			line: "loki=something,level=notlevel",
			err:  true,
		},
		{
			line: "loki=something,unknownoption",
			err:  true,
		},
		{
			line: "loki=something,label=somethng",
			err:  true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.line, func(t *testing.T) {
			// no parallel because this is way too fast and parallel will only slow it down

			res, err := LokiFromConfigLine(context.Background(), nil, test.line)

			if test.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			test.res.client = res.(*lokiHook).client
			test.res.ch = res.(*lokiHook).ch
			require.Equal(t, &test.res, res)
		})
	}
}

func TestParseArray(t *testing.T) {
	cases := [...]struct {
		key, value string
		i          int
		args       []string
		result     []string
		resultI    int
		err        bool
	}{
		{
			key:     "test",
			value:   "[some",
			i:       2,
			args:    []string{"else=asa", "e=s", "test=[some", "else]"},
			result:  []string{"some", "else"},
			resultI: 3,
		},
		{
			key:   "test",
			value: "[some",
			i:     2,
			args:  []string{"else=asa", "e=s", "test=[some", "else"},
			err:   true,
		},
		{
			key:   "test",
			value: "[some",
			i:     2,
			args:  []string{"else=asa", "e=s", "test=[some", "", "s]"},
			err:   true,
		},
		{
			key:   "test",
			value: "some",
			i:     2,
			args:  []string{"else=asa", "e=s", "test=some", "else]"},
			err:   true,
		},
		{
			key:     "test",
			value:   "",
			i:       2,
			args:    []string{"else=asa", "e=s", "test=", "sdasa"},
			result:  []string{},
			resultI: 2,
		},
		{
			key:     "test",
			value:   "[some]",
			i:       2,
			args:    []string{"else=asa", "e=s", "test=[some]", "else=sa"},
			result:  []string{"some"},
			resultI: 2,
		},
	}

	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			result, i, err := parseArray(c.key, c.value, c.i, c.args)
			assert.Equal(t, c.result, result)
			assert.Equal(t, c.resultI, i)
			if c.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLogEntryMarshal(t *testing.T) {
	entry := logEntry{
		t:   9223372036854775807, // the actual max
		msg: "something",
	}
	expected := []byte(`["9223372036854775807","something"]`)
	s, err := json.Marshal(entry)
	require.NoError(t, err)

	require.Equal(t, expected, s)
}
