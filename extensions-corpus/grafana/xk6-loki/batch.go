package loki

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	fake "github.com/brianvoe/gofakeit/v6"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
	json "github.com/mailru/easyjson"
	"github.com/prometheus/common/model"
	"go.k6.io/k6/js/common"
)

var LabelValuesFormat = []string{"apache_common", "apache_combined", "apache_error", "rfc3164", "rfc5424", "json", "logfmt"}

type FakeFunc func() string

type LabelPool map[model.LabelName][]string

type Batch struct {
	Streams   map[string]*push.Stream
	Bytes     int
	CreatedAt time.Time
}

type Entry struct {
	push.Entry
	TenantID string
	Labels   model.LabelSet
}

//go:generate easyjson -pkg -no_std_marshalers -gen_build_flags -mod=mod .
//easyjson:json
type JSONStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

//easyjson:json
type JSONPushRequest struct {
	Streams []JSONStream `json:"streams"`
}

func isValidLogFormat(format string) bool {
	for _, f := range LabelValuesFormat {
		if f == format {
			return true
		}
	}
	return false
}

// encodeSnappy encodes the batch as snappy-compressed push request, and
// returns the encoded bytes and the number of encoded entries
func (b *Batch) encodeSnappy() ([]byte, int, error) {
	req, entriesCount := b.createPushRequest()
	buf, err := proto.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, entriesCount, nil
}

// encodeJSON encodes the batch as JSON push request, and returns the encoded
// bytes and the number of encoded entries
func (b *Batch) encodeJSON() ([]byte, int, error) {
	req, entriesCount := b.createJSONPushRequest()
	buf, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	return buf, entriesCount, nil
}

// createJSONPushRequest creates a JSON push payload and returns it, together with
// number of entries
func (b *Batch) createJSONPushRequest() (*JSONPushRequest, int) {
	req := JSONPushRequest{
		Streams: make([]JSONStream, 0, len(b.Streams)),
	}

	entriesCount := 0
	for _, stream := range b.Streams {
		req.Streams = append(req.Streams, JSONStream{
			Stream: labelStringToMap(stream.Labels),
			Values: entriesToValues(stream.Entries),
		})
		entriesCount += len(stream.Entries)
	}
	return &req, entriesCount
}

// labelStringToMap converts a label string used by the `Batch` struct in
// format `{label_a="value_a",label_b="value_b"}` to a map that can be used in the
// JSON payload of push requests.
func labelStringToMap(labels string) map[string]string {
	kvList := strings.Trim(labels, "{}")
	kv := strings.Split(kvList, ",")
	labelMap := make(map[string]string, len(kv))
	for _, item := range kv {
		parts := strings.Split(item, "=")
		labelMap[parts[0]] = parts[1][1 : len(parts[1])-1]
	}
	return labelMap
}

// entriesToValues converts a slice of `Entry` to a slice of string tuples that
// can be used in the JSON payload of push requests.
func entriesToValues(entries []push.Entry) [][]string {
	lines := make([][]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, []string{
			strconv.FormatInt(entry.Timestamp.UnixNano(), 10),
			entry.Line,
		})
	}
	return lines
}

// createPushRequest creates a push request and returns it, together with
// number of entries
func (b *Batch) createPushRequest() (*push.PushRequest, int) {
	req := push.PushRequest{
		Streams: make([]push.Stream, 0, len(b.Streams)),
	}

	entriesCount := 0
	for _, stream := range b.Streams {
		req.Streams = append(req.Streams, *stream)
		entriesCount += len(stream.Entries)
	}
	return &req, entriesCount
}

// getRandomLabelSet creates a label set from the possible Client labels
func (c *Client) getRandomLabelSet() model.LabelSet {
	// TODO: improve it so there is no possibility to return duplicate label sets for the same batch?
	ls := make(model.LabelSet, len(c.labels))
	for _, label := range c.labels {
		ls[label.name] = model.LabelValue(label.values[c.rand.Intn(len(label.values))])
	}
	return ls
}

// newBatch creates a batch with randomly generated log streams
func (c *Client) newBatch(numStreams, minBatchSize, maxBatchSize int) *Batch {
	batch := &Batch{
		Streams:   make(map[string]*push.Stream, numStreams),
		CreatedAt: time.Now(),
	}
	state := c.vu.State()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}

	maxSizePerStream := minBatchSize
	if minBatchSize != maxBatchSize {
		maxSizePerStream += c.rand.Intn(maxBatchSize - minBatchSize)
	}
	maxSizePerStream /= numStreams

	for i := 0; i < numStreams; i++ {
		labels := c.getRandomLabelSet()
		if _, ok := labels[model.InstanceLabel]; !ok {
			labels[model.InstanceLabel] = model.LabelValue(fmt.Sprintf("vu%d.%s", state.VUID, hostname))
		}
		stream := &push.Stream{Labels: labels.String()}
		batch.Streams[stream.Labels] = stream

		var now time.Time
		logFmt := string(labels[model.LabelName("format")])
		if !isValidLogFormat(logFmt) {
			common.Throw(c.vu.Runtime(), fmt.Errorf("%s is not a valid log format", logFmt))
		}
		var line string

		// We have batch.Bytes so far, and each stream is allotted around
		// maxSizePerStream, so our final byte this stream should be:
		streamMaxByte := maxSizePerStream * (i + 1)
		for ; batch.Bytes < streamMaxByte; batch.Bytes += len(line) {
			now = time.Now()
			line = c.flog.LogLine(logFmt, now)
			stream.Entries = append(stream.Entries, push.Entry{
				Timestamp: now,
				Line:      line,
			})
		}
	}

	return batch
}

// generateValues returns `n` label values generated with the `ff` gofakeit function
func generateValues(ff FakeFunc, n int) []string {
	res := make([]string, n)
	for i := 0; i < n; i++ {
		res[i] = ff()
	}
	return res
}

// newLabelPool creates a "pool" of values for each label name
func newLabelPool(faker *fake.Faker, cardinalities map[string]int) LabelPool {
	// TODO: fix so that other cardinalities values work
	lb := LabelPool{
		"format": []string{"apache_common", "apache_combined", "apache_error", "rfc3164", "rfc5424", "json", "logfmt"}, // needs to match the available flog formats
		"os":     []string{"darwin", "linux", "windows"},
	}
	if n, ok := cardinalities["namespace"]; ok {
		lb["namespace"] = generateValues(faker.BS, n)
	}
	if n, ok := cardinalities["app"]; ok {
		lb["app"] = generateValues(faker.AppName, n)
	}
	if n, ok := cardinalities["pod"]; ok {
		lb["pod"] = generateValues(faker.BS, n)
	}
	if n, ok := cardinalities["language"]; ok {
		lb["language"] = generateValues(faker.LanguageAbbreviation, n)
	}
	if n, ok := cardinalities["word"]; ok {
		lb["word"] = generateValues(faker.Noun, n)
	}
	return lb
}

func transformLabelPool(pool LabelPool) []labelValues {
	keys := make([]string, 0, len(pool))
	for k := range pool {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	result := make([]labelValues, len(pool))
	for i, k := range keys {
		ln := model.LabelName(k)
		result[i] = labelValues{
			name:   ln,
			values: pool[ln],
		}
	}
	return result
}
