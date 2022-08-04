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
				droppedMsg:    "k6 dropped %d log messages because they were above the limit of %d messages / %s",
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
				droppedMsg:    "k6 dropped %d log messages because they were above the limit of %d messages / %s foo=bar level=warning",
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

			res, err := LokiFromConfigLine(context.Background(), nil, test.line, make(chan struct{}))

			if test.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			test.res.client = res.(*lokiHook).client
			test.res.ch = res.(*lokiHook).ch
			test.res.lokiStopped = res.(*lokiHook).lokiStopped
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

func TestFilterLabels(t *testing.T) {
	cases := []struct {
		allowedLabels  []string
		labels         map[string]string
		expectedLabels map[string]string
		msg            string
		result         string
	}{
		{
			allowedLabels:  []string{"a", "b"},
			labels:         map[string]string{"a": "1", "b": "2", "d": "3", "c": "4", "e": "5"},
			expectedLabels: map[string]string{"a": "1", "b": "2"},
			msg:            "some msg",
			result:         "some msg c=4 d=3 e=5",
		},
		{
			allowedLabels:  []string{"a", "b"},
			labels:         map[string]string{"d": "3", "c": "4", "e": "5"},
			expectedLabels: map[string]string{},
			msg:            "some msg",
			result:         "some msg c=4 d=3 e=5",
		},
		{
			allowedLabels:  []string{"a", "b"},
			labels:         map[string]string{"a": "1", "d": "3", "c": "4", "e": "5"},
			expectedLabels: map[string]string{"a": "1"},
			msg:            "some msg",
			result:         "some msg c=4 d=3 e=5",
		},
	}

	for i, c := range cases {
		c := c
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			h := &lokiHook{}
			h.allowedLabels = c.allowedLabels
			result := h.filterLabels(c.labels, c.msg)
			assert.Equal(t, c.result, result)
			assert.Equal(t, c.expectedLabels, c.labels)
		})
	}
}
