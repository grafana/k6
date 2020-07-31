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
	addr       string
	labels     [][2]string
	ch         chan *logrus.Entry
	limit      int
	msgMaxSize int
	levels     []logrus.Level
	pushPeriod time.Duration
	client     *http.Client
	ctx        context.Context
	profile    bool
}

// LokiFromConfigLine returns a new logrus.Hook that pushes logrus.Entrys to loki and is configured
// through the provided line
func LokiFromConfigLine(ctx context.Context, line string) (logrus.Hook, error) {
	h := &lokiHook{
		addr:       "http://127.0.0.1:3100/loki/api/v1/push",
		limit:      100,
		levels:     logrus.AllLevels,
		pushPeriod: time.Second * 1,
		ctx:        ctx,
		msgMaxSize: 1024 * 1024, // 1mb
		ch:         make(chan *logrus.Entry, 1000),
	}
	if line == "loki" {
		return h, nil
	}

	parts := strings.SplitN(line, "=", 2)
	if parts[0] != "loki" {
		return nil, fmt.Errorf("loki configuration should be in the form `loki=url-to-push` but is `%s`", line)
	}
	args := strings.Split(parts[1], ",")
	h.addr = args[0]
	// TODO use something better ... maybe
	// https://godoc.org/github.com/kubernetes/helm/pkg/strvals
	// atleast until https://github.com/loadimpact/k6/issues/926?
	if len(args) == 1 {
		return h, nil
	}

	for _, arg := range args[1:] {
		paramParts := strings.SplitN(arg, "=", 2)

		if len(paramParts) != 2 {
			return nil, fmt.Errorf("loki arguments should be in the form `address,key1=value1,key2=value2`, got %s", arg)
		}

		key := paramParts[0]
		value := paramParts[1]

		var err error
		switch key {
		case "pushPeriod":
			h.pushPeriod, err = time.ParseDuration(value)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse the loki pushPeriod %w", err)
			}
		case "profile":
			h.profile = true
		case "limit":
			h.limit, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse the loki limit as a number %w", err)
			}
			if !(h.limit > 0) {
				return nil, fmt.Errorf("loki limit needs to be a posstive number, is %d", h.limit)
			}
		case "msgMaxSize":
			h.msgMaxSize, err = strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse the loki msgMaxSize as a number %w", err)
			}
			if !(h.msgMaxSize > 0) {
				return nil, fmt.Errorf("loki msgMaxSize needs to be a posstive number, is %d", h.msgMaxSize)
			}
		case "level":
			h.levels, err = getLevels(value)
			if err != nil {
				return nil, err
			}
		default:
			if strings.HasPrefix(key, "label.") {
				labelKey := strings.TrimPrefix(key, "label.")
				h.labels = append(h.labels, [2]string{labelKey, value})
				continue
			}

			return nil, fmt.Errorf("unknown loki config key %s", key)
		}
	}

	h.client = &http.Client{Timeout: h.pushPeriod}

	go h.loop()
	return h, nil
}

func getLevels(level string) ([]logrus.Level, error) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("unknown log level %s", level) // specifically use a custom error
	}
	index := sort.Search(len(logrus.AllLevels), func(i int) bool {
		return logrus.AllLevels[i] > lvl
	})
	return logrus.AllLevels[:index], nil
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
		oldLogs := make([]tmpMsg, 0, h.limit*2)
		for ch := range pushCh {
			msgsToPush, msgs = msgs, msgsToPush
			oldCount, oldDropped := count, dropped
			count, dropped = 0, 0
			cutOff := <-ch
			close(ch) // signal that more buffering can continue

			copy(oldLogs[len(oldLogs):len(oldLogs)+oldCount], msgsToPush[:oldCount])
			oldLogs = oldLogs[:len(oldLogs)+oldCount]

			t := time.Now()
			cutOffIndex := sortAndSplitMsgs(oldLogs, cutOff)
			if cutOffIndex == 0 {
				continue
			}
			t1 := time.Since(t)

			strms := h.createPushMessage(oldLogs, cutOffIndex, oldDropped)
			if cutOffIndex > len(oldLogs) {
				oldLogs = oldLogs[:0]
				continue
			}
			oldLogs = oldLogs[:copy(oldLogs, oldLogs[cutOffIndex:])]
			t2 := time.Since(t) - t1

			var b bytes.Buffer
			_, err := strms.WriteTo(&b)
			if err != nil {
				fmt.Printf("Error while marshaling logs for loki %s\n", err)
				continue
			}
			size := b.Len()
			t3 := time.Since(t) - t2 - t1

			err = h.push(b)
			if err != nil {
				fmt.Printf("Error while sending logs to loki %s\n", err)
				continue
			}
			t4 := time.Since(t) - t3 - t2 - t1

			if h.profile {
				fmt.Printf("sorting=%s, adding=%s marshalling=%s sending=%s count=%d final_size=%d\n",
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
			// have the cutoff here ?
			// if we cutoff here we can cut somewhat on the backbuffers and optimize the inserting
			// in/creating of the final Streams that we push
			msgs[count] = tmpMsg{
				labels: labels,
				msg:    entry.Message,
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
			ch <- 0
			<-ch
			return
		}
	}
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
	strms := new(lokiPushMessage)
	strms.msgMaxSize = h.msgMaxSize
	for _, msg := range msgs[:cutOffIndex] {
		strms.add(msg)
	}
	if dropped != 0 {
		labels := make(map[string]string, 2+len(h.labels))
		labels["level"] = logrus.WarnLevel.String()
		labels["dropped"] = strconv.Itoa(dropped)
		for _, params := range h.labels {
			labels[params[0]] = params[1]
		}

		msg := tmpMsg{
			labels: labels,
			msg: fmt.Sprintf("k6 dropped some log messages because they were above the limit of %d/%s",
				h.limit, h.pushPeriod),
			t: msgs[cutOffIndex-1].t,
		}
		strms.add(msg)
	}
	return strms
}

func (h *lokiHook) push(b bytes.Buffer) error {
	body := b.Bytes()

	req, err := http.NewRequestWithContext(context.Background(), "GET", h.addr, &b)
	if err != nil {
		return err
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewBuffer(body)), nil
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := h.client.Do(req)

	if res != nil {
		if res.StatusCode == 400 {
			r, _ := ioutil.ReadAll(res.Body) // maybe limit it to something like the first 1000 characters?
			return fmt.Errorf("Got 400 from loki: " + string(r))
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

func (strms *lokiPushMessage) add(entry tmpMsg) {
	var foundStrm *stream
	for _, strm := range strms.Streams {
		if mapEqual(strm.Stream, entry.labels) {
			foundStrm = strm
			break
		}
	}

	if foundStrm == nil {
		foundStrm = &stream{Stream: entry.labels}
		strms.Streams = append(strms.Streams, foundStrm)
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
	Streams    []*stream `json:"streams"`
	msgMaxSize int
}

func (strms *lokiPushMessage) WriteTo(w io.Writer) (n int64, err error) {
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
	for i, str := range strms.Streams {
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
			if len([]rune(v.msg)) > strms.msgMaxSize {
				difference := int64(len(v.msg) - strms.msgMaxSize)
				omitMsg := append(strconv.AppendInt([]byte("... omitting "), difference, 10), " characters ..."...)
				v.msg = strings.Join([]string{
					string([]rune(v.msg)[:strms.msgMaxSize/2]),
					string([]rune(v.msg)[len([]rune(v.msg))-strms.msgMaxSize/2:]),
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
