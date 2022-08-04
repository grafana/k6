package log

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type lokiHook struct {
	ctx            context.Context
	fallbackLogger logrus.FieldLogger
	lokiStopped    chan<- struct{}

	addr          string
	labels        [][2]string
	ch            chan *logrus.Entry
	limit         int
	msgMaxSize    int
	levels        []logrus.Level
	allowedLabels []string
	pushPeriod    time.Duration
	client        *http.Client
	profile       bool
	droppedLabels map[string]string
	droppedMsg    string
}

func getDefaultLoki() *lokiHook {
	return &lokiHook{
		addr:          "http://127.0.0.1:3100/loki/api/v1/push",
		limit:         100,
		levels:        logrus.AllLevels,
		pushPeriod:    time.Second * 1,
		msgMaxSize:    1024 * 1024, // 1mb
		ch:            make(chan *logrus.Entry, 1000),
		allowedLabels: nil,
		droppedMsg:    "k6 dropped %d log messages because they were above the limit of %d messages / %s",
	}
}

// LokiFromConfigLine returns a new logrus.Hook that pushes logrus.Entrys to loki and is configured
// through the provided line
//nolint:funlen
func LokiFromConfigLine(
	ctx context.Context, fallbackLogger logrus.FieldLogger, line string, ch chan<- struct{},
) (logrus.Hook, error) {
	h := getDefaultLoki()

	h.ctx = ctx
	h.lokiStopped = ch
	h.fallbackLogger = fallbackLogger

	if line != "loki" {
		parts := strings.SplitN(line, "=", 2)
		if parts[0] != "loki" {
			return nil, fmt.Errorf("loki configuration should be in the form `loki=url-to-push` but is `%s`", line)
		}

		err := h.parseArgs(line)
		if err != nil {
			return nil, err
		}
	}
	h.droppedLabels = make(map[string]string, 2+len(h.labels))
	h.droppedLabels["level"] = logrus.WarnLevel.String()
	for _, params := range h.labels {
		h.droppedLabels[params[0]] = params[1]
	}

	h.droppedMsg = h.filterLabels(h.droppedLabels, h.droppedMsg)

	h.client = &http.Client{Timeout: h.pushPeriod}

	go h.loop()

	return h, nil
}

func (h *lokiHook) parseArgs(line string) error {
	tokens, err := tokenize(line)
	if err != nil {
		return fmt.Errorf("error while parsing loki configuration %w", err)
	}

	for _, token := range tokens {
		key := token.key
		value := token.value

		var err error
		switch key {
		case "loki":
			h.addr = value
		case "pushPeriod":
			h.pushPeriod, err = time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("couldn't parse the loki pushPeriod %w", err)
			}
		case "profile":
			h.profile = true
		case "limit":
			h.limit, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("couldn't parse the loki limit as a number %w", err)
			}
			if !(h.limit > 0) {
				return fmt.Errorf("loki limit needs to be a positive number, is %d", h.limit)
			}
		case "msgMaxSize":
			h.msgMaxSize, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("couldn't parse the loki msgMaxSize as a number %w", err)
			}
			if !(h.msgMaxSize > 0) {
				return fmt.Errorf("loki msgMaxSize needs to be a positive number, is %d", h.msgMaxSize)
			}
		case "level":
			h.levels, err = parseLevels(value)
			if err != nil {
				return err
			}
		case "allowedLabels":
			h.allowedLabels = strings.Split(value, ",")
		default:
			if strings.HasPrefix(key, "label.") {
				labelKey := strings.TrimPrefix(key, "label.")
				h.labels = append(h.labels, [2]string{labelKey, value})

				continue
			}

			return fmt.Errorf("unknown loki config key %s", key)
		}
	}

	return nil
}

// fill one of two equally sized slices with entries and then push it while filling the other one
// TODO benchmark this
//nolint:funlen
func (h *lokiHook) loop() {
	var (
		msgs       = make([]tmpMsg, h.limit)
		msgsToPush = make([]tmpMsg, h.limit)
		dropped    int
		count      int
		ticker     = time.NewTicker(h.pushPeriod)
		pushCh     = make(chan chan int64)
	)

	defer ticker.Stop()
	defer close(pushCh)

	go func() {
		defer close(h.lokiStopped)

		oldLogs := make([]tmpMsg, 0, h.limit*2)
		for ch := range pushCh {
			msgsToPush, msgs = msgs, msgsToPush
			oldCount, oldDropped := count, dropped
			count, dropped = 0, 0
			cutOff := <-ch
			close(ch) // signal that more buffering can continue

			oldLogs = append(oldLogs, msgsToPush[:oldCount]...)

			t := time.Now()
			cutOffIndex := sortAndSplitMsgs(oldLogs, cutOff)
			if cutOffIndex == 0 {
				continue
			}
			t1 := time.Since(t)

			pushMsg := h.createPushMessage(oldLogs, cutOffIndex, oldDropped)
			if cutOffIndex > len(oldLogs) {
				oldLogs = oldLogs[:0]

				continue
			}
			oldLogs = oldLogs[:copy(oldLogs, oldLogs[cutOffIndex:])]
			t2 := time.Since(t) - t1

			var b bytes.Buffer
			_, err := pushMsg.WriteTo(&b)
			if err != nil {
				h.fallbackLogger.WithError(err).Error("Error while marshaling logs for loki")

				continue
			}
			size := b.Len()
			t3 := time.Since(t) - t2 - t1

			err = h.push(b)
			if err != nil {
				h.fallbackLogger.WithError(err).Error("Error while sending logs to loki")

				continue
			}
			t4 := time.Since(t) - t3 - t2 - t1

			if h.profile {
				h.fallbackLogger.Infof(
					"sorting=%s, adding=%s marshalling=%s sending=%s count=%d final_size=%d\n",
					t1, t2, t3, t4, cutOffIndex, size)
			}
		}
	}()

	for {
		select {
		case entry := <-h.ch:
			if count == h.limit {
				dropped++

				continue
			}

			// Arguably we can directly generate the final marshalled version of the labels right here
			// through sorting the entry.Data, removing additionalparams from it and then dumping it
			// as the final marshal and appending level and h.labels after it.
			// If we reuse some kind of big enough `[]byte` buffer we can also possibly skip on some
			// of allocation. Combined with the cutoff part and directly pushing in the final data
			// type this can be really a lot faster and to use a lot less memory
			labels := make(map[string]string, len(entry.Data)+1)
			for k, v := range entry.Data {
				labels[k] = fmt.Sprint(v) // TODO optimize ?
			}
			for _, params := range h.labels {
				labels[params[0]] = params[1]
			}
			labels["level"] = entry.Level.String()
			msg := h.filterLabels(labels, entry.Message) // TODO we can do this while constructing
			// have the cutoff here ?
			// if we cutoff here we can cut somewhat on the backbuffers and optimize the inserting
			// in/creating of the final Streams that we push
			msgs[count] = tmpMsg{
				labels: labels,
				msg:    msg,
				t:      entry.Time.UnixNano(),
			}
			count++
		case t := <-ticker.C:
			ch := make(chan int64)
			pushCh <- ch
			ch <- t.Add(-(h.pushPeriod / 2)).UnixNano()
			<-ch
		case <-h.ctx.Done():
			ch := make(chan int64)
			pushCh <- ch
			ch <- time.Now().Add(time.Second).UnixNano()
			<-ch

			return
		}
	}
}

func (h *lokiHook) filterLabels(labels map[string]string, msg string) string {
	if h.allowedLabels == nil {
		return msg
	}
	// TODO both can be reused as under load this will just generate a lot of *probably* fairly
	// similar objects.
	var b strings.Builder
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	b.WriteString(msg)
outer:
	for _, key := range keys {
		for _, label := range h.allowedLabels {
			if label == key {
				continue outer
			}
		}
		b.WriteRune(' ')
		b.WriteString(key)
		b.WriteRune('=')
		b.WriteString(labels[key])
		delete(labels, key)
	}

	return b.String()
}

func sortAndSplitMsgs(msgs []tmpMsg, cutOff int64) int {
	if len(msgs) == 0 {
		return 0
	}

	// TODO using time.Before was giving a lot of out of order, but even now, there are some, if the
	// limit is big enough ...
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].t < msgs[j].t
	})

	cutOffIndex := sort.Search(len(msgs), func(i int) bool {
		return !(msgs[i].t < cutOff)
	})

	return cutOffIndex
}

func (h *lokiHook) createPushMessage(msgs []tmpMsg, cutOffIndex, dropped int) *lokiPushMessage {
	pushMsg := new(lokiPushMessage)
	pushMsg.maxSize = h.msgMaxSize
	for _, msg := range msgs[:cutOffIndex] {
		pushMsg.add(msg)
	}
	if dropped != 0 {
		msg := tmpMsg{
			labels: h.droppedLabels,
			msg:    fmt.Sprintf(h.droppedMsg, dropped, h.limit, h.pushPeriod),
			t:      msgs[cutOffIndex-1].t,
		}
		pushMsg.add(msg)
	}

	return pushMsg
}

func (h *lokiHook) push(b bytes.Buffer) error {
	body := b.Bytes()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, h.addr, &b)
	if err != nil {
		return err
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewBuffer(body)), nil
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := h.client.Do(req)

	if res != nil {
		if res.StatusCode >= 400 {
			r, _ := ioutil.ReadAll(res.Body) // maybe limit it to something like the first 1000 characters?

			return fmt.Errorf("got %d from loki: %s", res.StatusCode, string(r))
		}
		_, _ = io.Copy(ioutil.Discard, res.Body)
		_ = res.Body.Close()
	}

	return err
}

func mapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if v2, ok := b[k]; !ok || v2 != v {
			return false
		}
	}

	return true
}

func (pushMsg *lokiPushMessage) add(entry tmpMsg) {
	var foundStrm *stream
	for _, strm := range pushMsg.Streams {
		if mapEqual(strm.Stream, entry.labels) {
			foundStrm = strm

			break
		}
	}

	if foundStrm == nil {
		foundStrm = &stream{Stream: entry.labels}
		pushMsg.Streams = append(pushMsg.Streams, foundStrm)
	}

	foundStrm.Values = append(foundStrm.Values, logEntry{t: entry.t, msg: entry.msg})
}

// this is temporary message format used to not keep the logrus.Entry around too long and to make
// sorting easier
type tmpMsg struct {
	labels map[string]string
	t      int64
	msg    string
}

func (h *lokiHook) Fire(entry *logrus.Entry) error {
	h.ch <- entry

	return nil
}

func (h *lokiHook) Levels() []logrus.Level {
	return h.levels
}

/*
{
  "streams": [
    {
      "stream": {
        "label1": "value1"
        "label2": "value2"
      },
      "values": [ // the nanoseconds need to be in order
          [ "<unix epoch in nanoseconds>", "<log line>" ],
          [ "<unix epoch in nanoseconds>", "<log line>" ]
      ]
    }
  ]
}
*/
type lokiPushMessage struct {
	Streams []*stream `json:"streams"`
	maxSize int
}

func (pushMsg *lokiPushMessage) WriteTo(w io.Writer) (n int64, err error) {
	var k int
	write := func(b []byte) {
		if err != nil {
			return
		}
		k, err = w.Write(b)
		n += int64(k)
	}
	// 10+ 9 for the amount of nanoseconds between 2001 and 2286 also it overflows in the year 2262 ;)
	var nanoseconds [19]byte
	write([]byte(`{"streams":[`))
	var b []byte
	for i, str := range pushMsg.Streams {
		if i != 0 {
			write([]byte(`,`))
		}
		write([]byte(`{"stream":{`))
		var f bool
		for k, v := range str.Stream {
			if f {
				write([]byte(`,`))
			}
			f = true
			write([]byte(`"`))
			write([]byte(k))
			write([]byte(`":`))
			b, err = json.Marshal(v)
			if err != nil {
				return n, err
			}
			write(b)
		}
		write([]byte(`},"values":[`))
		for j, v := range str.Values {
			if j != 0 {
				write([]byte(`,`))
			}
			write([]byte(`["`))
			strconv.AppendInt(nanoseconds[:0], v.t, 10)
			write(nanoseconds[:])
			write([]byte(`",`))
			msgRunes := []rune(v.msg)
			if len(msgRunes) > pushMsg.maxSize {
				difference := int64(len(msgRunes) - pushMsg.maxSize)
				omitMsg := append(strconv.AppendInt([]byte("... omitting "), difference, 10), " characters ..."...)
				v.msg = strings.Join([]string{
					string(msgRunes[:pushMsg.maxSize/2]),
					string(msgRunes[len(msgRunes)-pushMsg.maxSize/2:]),
				}, string(omitMsg))
			}

			b, err = json.Marshal(v.msg)
			if err != nil {
				return n, err
			}
			write(b)
			write([]byte(`]`))
		}
		write([]byte(`]}`))
	}

	write([]byte(`]}`))

	return n, err
}

type stream struct {
	Stream map[string]string `json:"stream"`
	Values []logEntry        `json:"values"`
}

type logEntry struct {
	t   int64  // nanoseconds
	msg string // maybe intern those as they are likely to be the same for an interval
}

// rewrite this either with easyjson or with a custom marshalling
func (l logEntry) MarshalJSON() ([]byte, error) {
	// 2 for '[]', 1 for ',', 4 for '"' and 10 + 9 for the amount of nanoseconds between 2001 and
	// 2286 also it overflows in the year 2262 ;)
	b := make([]byte, 2, len(l.msg)+26)
	b[0] = '['
	b[1] = '"'
	b = strconv.AppendInt(b, l.t, 10)
	b = append(b, '"', ',', '"')
	b = append(b, l.msg...)
	b = append(b, '"', ']')

	return b, nil
}
