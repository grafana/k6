package cloudapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
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

func TestRetry(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			attempts int
			expWaits []time.Duration // pow(abs(interval), attempt index)
		}{
			{
				name:     "NoRetry",
				attempts: 1,
			},
			{
				name:     "TwoAttempts",
				attempts: 2,
				expWaits: []time.Duration{5 * time.Second},
			},
			{
				name:     "MaximumExceeded",
				attempts: 4,
				expWaits: []time.Duration{5 * time.Second, 25 * time.Second, 2 * time.Minute},
			},
			{
				name:     "AttemptsLimit",
				attempts: 5,
				expWaits: []time.Duration{5 * time.Second, 25 * time.Second, 2 * time.Minute, 2 * time.Minute},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var sleepRequests []time.Duration
				// sleepCollector tracks the request duration value for sleep requests.
				sleepCollector := sleeperFunc(func(d time.Duration) {
					sleepRequests = append(sleepRequests, d)
				})

				var iterations int
				err := retry(sleepCollector, 5, 5*time.Second, 2*time.Minute, func() error {
					iterations++
					if iterations < tt.attempts {
						return fmt.Errorf("unexpected error")
					}
					return nil
				})
				require.NoError(t, err)
				require.Equal(t, tt.attempts, iterations)
				require.Equal(t, len(tt.expWaits), len(sleepRequests))

				// the added random milliseconds makes difficult to know the exact value
				// so it asserts that expwait <= actual <= expwait + 1s
				for i, expwait := range tt.expWaits {
					assert.GreaterOrEqual(t, sleepRequests[i], expwait)
					assert.LessOrEqual(t, sleepRequests[i], expwait+(1*time.Second))
				}
			})
		}
	})
	t.Run("Fail", func(t *testing.T) {
		t.Parallel()

		mock := sleeperFunc(func(time.Duration) { /* noop - nowait */ })
		err := retry(mock, 5, 5*time.Second, 30*time.Second, func() error {
			return fmt.Errorf("unexpected error")
		})

		assert.Error(t, err, "unexpected error")
	})
}

func TestStreamLogsToLogger(t *testing.T) {
	t.Parallel()

	// It registers an handler for the logtail endpoint
	// It upgrades as websocket the HTTP handler and invokes the provided callback.
	logtailHandleFunc := func(tb *httpmultibin.HTTPMultiBin, fn func(*websocket.Conn, *http.Request)) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		tb.Mux.HandleFunc("/api/v1/tail", func(w http.ResponseWriter, req *http.Request) {
			conn, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)

			fn(conn, req)
			_ = conn.Close()
		})
	}

	// a basic config with the logtail endpoint set
	configFromHTTPMultiBin := func(tb *httpmultibin.HTTPMultiBin) Config {
		wsurl := strings.TrimPrefix(tb.ServerHTTP.URL, "http://")
		return Config{
			LogsTailURL: null.NewString(fmt.Sprintf("ws://%s/api/v1/tail", wsurl), false),
		}
	}

	// get all messages from the mocked logger
	logLines := func(hook *testutils.SimpleLogrusHook) (lines []string) {
		for _, e := range hook.Drain() {
			lines = append(lines, e.Message)
		}
		return
	}

	generateLogline := func(key string, ts uint64, msg string) string {
		return fmt.Sprintf(`{"streams":[{"stream":{"key":%q,"level":"warn"},"values":[["%d",%q]]}],"dropped_entities":[]}`, key, ts, msg)
	}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tb := httpmultibin.NewHTTPMultiBin(t)
		logtailHandleFunc(tb, func(conn *websocket.Conn, _ *http.Request) {
			rawmsg := json.RawMessage(generateLogline("stream1", 1598282752000000000, "logline1"))
			err := conn.WriteJSON(rawmsg)
			require.NoError(t, err)

			rawmsg = json.RawMessage(generateLogline("stream2", 1598282752000000001, "logline2"))
			err = conn.WriteJSON(rawmsg)
			require.NoError(t, err)

			// wait the flush on the network
			time.Sleep(5 * time.Millisecond)
			cancel()
		})

		logger := logrus.New()
		logger.Out = ioutil.Discard
		hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
		logger.AddHook(hook)

		c := configFromHTTPMultiBin(tb)
		err := c.StreamLogsToLogger(ctx, logger, "ref_id", 0)
		require.NoError(t, err)

		assert.Equal(t, []string{"logline1", "logline2"}, logLines(hook))
	})

	t.Run("RestoreConnFromLatestMessage", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		startFilter := func(u url.URL) (start time.Time, err error) {
			rawstart, err := strconv.ParseInt(u.Query().Get("start"), 10, 64)
			if err != nil {
				return start, err
			}

			start = time.Unix(0, rawstart)
			return
		}

		var requestsCount uint64

		tb := httpmultibin.NewHTTPMultiBin(t)
		logtailHandleFunc(tb, func(conn *websocket.Conn, req *http.Request) {
			requests := atomic.AddUint64(&requestsCount, 1)

			start, err := startFilter(*req.URL)
			require.NoError(t, err)

			if requests <= 1 {
				t0 := time.Date(2021, time.July, 27, 0, 0, 0, 0, time.UTC).UnixNano()
				t1 := time.Date(2021, time.July, 27, 1, 0, 0, 0, time.UTC).UnixNano()
				t2 := time.Date(2021, time.July, 27, 2, 0, 0, 0, time.UTC).UnixNano()

				// send a correct logline so we will able to assert
				// that the connection is restored from t2 as expected
				rawmsg := json.RawMessage(fmt.Sprintf(`{"streams":[{"stream":{"key":"stream1","level":"warn"},"values":[["%d","newest logline"],["%d","second logline"],["%d","oldest logline"]]}],"dropped_entities":[]}`, t2, t1, t0))
				err = conn.WriteJSON(rawmsg)
				require.NoError(t, err)

				// wait the flush of the message on the network
				time.Sleep(20 * time.Millisecond)

				// it generates a failure closing the connection
				// in a rude way
				err = conn.Close()
				require.NoError(t, err)
				return
			}

			// assert that the client created the request with `start`
			// populated from the most recent seen value (t2+1ns)
			require.Equal(t, time.Unix(0, 1627351200000000001), start)

			// send a correct logline so we will able to assert
			// that the connection is restored as expected
			err = conn.WriteJSON(json.RawMessage(generateLogline("stream3", 1627358400000000000, "logline-after-restored-conn")))
			require.NoError(t, err)

			// wait the flush of the message on the network
			time.Sleep(20 * time.Millisecond)
			cancel()
		})

		logger := logrus.New()
		logger.Out = ioutil.Discard
		hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
		logger.AddHook(hook)

		c := configFromHTTPMultiBin(tb)
		err := c.StreamLogsToLogger(ctx, logger, "ref_id", 0)
		require.NoError(t, err)

		assert.Equal(t,
			[]string{
				"newest logline",
				"second logline",
				"oldest logline",
				"error reading a log message from the cloud, trying to establish a fresh connection with the logs service...",
				"logline-after-restored-conn",
			}, logLines(hook))
	})

	t.Run("RestoreConnFromTimeNow", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		startFilter := func(u url.URL) (start time.Time, err error) {
			rawstart, err := strconv.ParseInt(u.Query().Get("start"), 10, 64)
			if err != nil {
				return start, err
			}

			start = time.Unix(0, rawstart)
			return
		}

		var requestsCount uint64
		t0 := time.Now()

		tb := httpmultibin.NewHTTPMultiBin(t)
		logtailHandleFunc(tb, func(conn *websocket.Conn, req *http.Request) {
			requests := atomic.AddUint64(&requestsCount, 1)

			start, err := startFilter(*req.URL)
			require.NoError(t, err)

			if requests <= 1 {
				// if it's the first attempt then
				// it generates a failure closing the connection
				// in a rude way
				err = conn.Close()
				require.NoError(t, err)
				return
			}

			// it asserts that the second attempt
			// has a `start` after the test run
			require.True(t, start.After(t0))

			// send a correct logline so we will able to assert
			// that the connection is restored as expected
			err = conn.WriteJSON(json.RawMessage(`{"streams":[{"stream":{"key":"stream1","level":"warn"},"values":[["1598282752000000000","logline-after-restored-conn"]]}],"dropped_entities":[]}`))
			require.NoError(t, err)

			// wait the flush of the message on the network
			time.Sleep(20 * time.Millisecond)
			cancel()
		})

		logger := logrus.New()
		logger.Out = ioutil.Discard
		hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
		logger.AddHook(hook)

		c := configFromHTTPMultiBin(tb)
		err := c.StreamLogsToLogger(ctx, logger, "ref_id", 0)
		require.NoError(t, err)

		assert.Equal(t,
			[]string{
				"error reading a log message from the cloud, trying to establish a fresh connection with the logs service...",
				"logline-after-restored-conn",
			}, logLines(hook))
	})
}
