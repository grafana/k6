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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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
				ctx:        context.Background(),
				addr:       "http://127.0.0.1:3100/loki/api/v1/push",
				limit:      100,
				pushPeriod: time.Second * 1,
				levels:     logrus.AllLevels,
			},
		},
		{
			line: "loki=somewhere:1233,label.something=else,label.foo=bar,limit=32,level=debug,pushPeriod=5m32s",
			res: lokiHook{
				ctx:        context.Background(),
				addr:       "somewhere:1233",
				limit:      32,
				pushPeriod: time.Minute*5 + time.Second*32,
				levels:     []logrus.Level{logrus.DebugLevel, logrus.InfoLevel, logrus.WarnLevel, logrus.ErrorLevel},
				labels:     [][2]string{{"something", "else"}, {"foo", "bar"}},
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

			res, err := LokiFromConfigLine(context.Background(), test.line)

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
