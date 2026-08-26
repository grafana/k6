// Package remotewrite provides a k6 extension for sending metrics to Prometheus Remote Write endpoints.
// This extension allows k6 load tests to generate and send time series data to any Prometheus-compatible
// remote write endpoint, enabling performance testing of metric ingestion pipelines.
package remotewrite

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/sobek"
	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/prompb"
	"github.com/xhit/go-str2duration/v2"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
)

var (
	// ErrInvalidConfig is returned when the client configuration is invalid.
	ErrInvalidConfig = errors.New("Client constructor expects first argument to be Config")
	// ErrURLRequired is returned when the URL is not provided in the configuration.
	ErrURLRequired = errors.New("url is required")
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/remotewrite".
func init() {
	modules.Register("k6/x/remotewrite", new(remoteWriteModule))
}

// RemoteWrite is the k6 extension for interacting Prometheus Remote Write endpoints.
type RemoteWrite struct {
	vu modules.VU
}

type remoteWriteModule struct{}

var _ modules.Module = &remoteWriteModule{}

func (r *remoteWriteModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &RemoteWrite{
		vu: vu,
	}
}

// Exports returns the exports of the module for k6.
func (r *RemoteWrite) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"Client":                   r.xclient,
			"Sample":                   r.sample,
			"Timeseries":               r.timeseries,
			"precompileLabelTemplates": compileLabelTemplates,
		},
	}
}

// Client is the client wrapper.
type Client struct {
	cfg *Config
	vu  modules.VU
}

// Config holds the configuration for the Prometheus Remote Write client.
type Config struct {
	Url        string            `json:"url"`        //nolint:revive // sobek exports value here
	UserAgent  string            `json:"user_agent"` //nolint:tagliatelle // sobek use snake case for JSON keys
	Timeout    string            `json:"timeout"`
	TenantName string            `json:"tenant_name"` //nolint:tagliatelle // sobek use snake case for JSON keys
	Headers    map[string]string `json:"headers"`
}

// xclient constructs a new Remote Write Client instance.
func (r *RemoteWrite) xclient(c sobek.ConstructorCall) *sobek.Object {
	var config Config

	rt := r.vu.Runtime()

	err := rt.ExportTo(c.Argument(0), &config)
	if err != nil {
		common.Throw(rt, ErrInvalidConfig)
	}

	if config.Url == "" {
		log.Fatal(ErrURLRequired)
	}

	if config.UserAgent == "" {
		config.UserAgent = "k6-remote-write/0.0.2"
	}

	if config.Timeout == "" {
		config.Timeout = "10s"
	}

	return rt.ToValue(&Client{
		cfg: &config,
		vu:  r.vu,
	}).ToObject(rt)
}

// Timeseries represents a Prometheus time series with labels and samples.
type Timeseries struct {
	Labels  []Label
	Samples []Sample
}

// Label represents a Prometheus label name-value pair.
type Label struct {
	Name, Value string
}

// Sample represents a single Prometheus sample with value and timestamp.
type Sample struct {
	Value     float64
	Timestamp int64
}

func (r *RemoteWrite) sample(c sobek.ConstructorCall) *sobek.Object {
	rt := r.vu.Runtime()
	call, _ := sobek.AssertFunction(rt.ToValue(xsample))

	v, err := call(sobek.Undefined(), c.Arguments...)
	if err != nil {
		common.Throw(rt, err)
	}

	return v.ToObject(rt)
}

func xsample(value float64, timestamp int64) Sample {
	return Sample{
		Value:     value,
		Timestamp: timestamp,
	}
}

func (r *RemoteWrite) timeseries(c sobek.ConstructorCall) *sobek.Object {
	rt := r.vu.Runtime()
	call, _ := sobek.AssertFunction(rt.ToValue(xtimeseries))

	v, err := call(sobek.Undefined(), c.Arguments...)
	if err != nil {
		common.Throw(rt, err)
	}

	return v.ToObject(rt)
}

func xtimeseries(labels map[string]string, samples []Sample) *Timeseries {
	t := &Timeseries{
		Labels:  make([]Label, 0, len(labels)),
		Samples: samples,
	}

	for k, v := range labels {
		t.Labels = append(t.Labels, Label{Name: k, Value: v})
	}

	return t
}

// StoreGenerated generates and stores synthetic time series data for load testing.
func (c *Client) StoreGenerated(totalSeries, batches, batchSize, batch int64) (httpext.Response, error) {
	ts, err := generateSeries(totalSeries, batches, batchSize, batch)
	if err != nil {
		return *httpext.NewResponse(), err
	}

	return c.Store(ts)
}

func generateSeries(totalSeries, batches, batchSize, batch int64) ([]Timeseries, error) {
	if totalSeries == 0 {
		return nil, nil
	}

	if batch > batches {
		return nil, errors.New("batch must be in the range of batches")
	}

	if totalSeries/batches != batchSize {
		return nil, errors.New("total_series must divide evenly into batches of size batch_size")
	}

	// #nosec G404 -- This is test data generation for load testing, not cryptographic use
	r := rand.New(rand.NewSource(time.Now().Unix()))
	series := make([]Timeseries, batchSize)
	timestamp := time.Now().UnixNano() / int64(time.Millisecond)

	for i := range batchSize {
		seriesID := batchSize*(batch-1) + i
		labels := generateCardinalityLabels(totalSeries, seriesID)
		labels = append(labels, Label{
			Name:  "__name__",
			Value: "k6_generated_metric_" + strconv.Itoa(int(seriesID)),
		})

		// Required for querying in order to have unique series excluding the metric name.
		labels = append(labels, Label{
			Name:  "series_id",
			Value: strconv.Itoa(int(seriesID)),
		})

		series[i] = Timeseries{
			labels,
			[]Sample{{r.Float64() * 100, timestamp}},
		}
	}

	return series, nil
}

func generateCardinalityLabels(totalSeries, seriesID int64) []Label {
	// exp is the greatest exponent of 10 that is less than total series.
	exp := int64(math.Log10(float64(totalSeries)))
	labels := make([]Label, 0, exp)

	for x := 1; int64(x) <= exp; x++ {
		labels = append(labels, Label{
			Name: "cardinality_1e" + strconv.Itoa(x),
			//nolint:mnd // 10 is the base for decimal exponentiation
			Value: strconv.Itoa(int(seriesID / int64(math.Pow(10, float64(x))))),
		})
	}

	return labels
}

// Store sends the provided time series to the Prometheus Remote Write endpoint.
func (c *Client) Store(ts []Timeseries) (httpext.Response, error) {
	batch := make([]prompb.TimeSeries, 0, len(ts))

	for _, t := range ts {
		batch = append(batch, FromTimeseriesToPrometheusTimeseries(t))
	}

	return c.store(batch)
}

// ResponseCallback checks if the HTTP status code indicates success (2xx).
func ResponseCallback(n int) bool {
	//nolint:mnd // 2 represents 2xx HTTP status codes
	return n/100 == 2
}

// FromTimeseriesToPrometheusTimeseries converts a Timeseries to a Prometheus TimeSeries.
func FromTimeseriesToPrometheusTimeseries(ts Timeseries) prompb.TimeSeries {
	labels := make([]prompb.Label, 0, len(ts.Labels))
	samples := make([]prompb.Sample, 0, len(ts.Samples))

	for _, label := range ts.Labels {
		labels = append(labels, prompb.Label{
			Name:  label.Name,
			Value: label.Value,
		})
	}

	for _, sample := range ts.Samples {
		if sample.Timestamp == 0 {
			sample.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
		}

		samples = append(samples, prompb.Sample{
			Value:     sample.Value,
			Timestamp: sample.Timestamp,
		})
	}

	return prompb.TimeSeries{
		Labels:  labels,
		Samples: samples,
	}
}

// The only supported things are:
// 1. replacing ${series_id} with the series_id provided.
// 2. replacing ${series_id/<integer>} with the evaluation of that.
// 3. if error in parsing return error.
func compileTemplate(template string) (*labelGenerator, error) {
	i := strings.Index(template, "${series_id")
	if i == -1 {
		return newIdentityLabelGenerator(template), nil
	}

	switch template[i+len("${series_id")] {
	case '}':
		return &labelGenerator{
			AppendByte: func(b []byte, seriesID int) []byte {
				b = append(b, template[:i]...)
				//nolint:mnd // 10 is the base for decimal string conversion
				b = strconv.AppendInt(b, int64(seriesID), 10)

				return append(b, template[i+len("${series_id}"):]...)
			},
		}, nil
	case '%':
		end := strings.Index(template[i:], "}")
		if end == -1 {
			return nil, errors.New("no closing bracket in template")
		}

		d, err := strconv.Atoi(template[i+len("${series_id%") : i+end])
		if err != nil {
			return nil, fmt.Errorf("can't parse divisor of the module operator %w", err)
		}

		possibleValues := make([][]byte, d)
		// REVIEW TODO have an upper limit
		for j := range d {
			var b []byte

			b = append(b, template[:i]...)
			//nolint:mnd // 10 is the base for decimal string conversion
			b = strconv.AppendInt(b, int64(j), 10)
			possibleValues[j] = append(b, template[i+end+1:]...)
		}

		return &labelGenerator{
			AppendByte: func(b []byte, seriesID int) []byte {
				return append(b, possibleValues[seriesID%d]...)
			},
		}, nil
	case '/':
		end := strings.Index(template[i:], "}")
		if end == -1 {
			return nil, errors.New("no closing bracket in template")
		}

		d, err := strconv.Atoi(template[i+len("${series_id/") : i+end])
		if err != nil {
			return nil, err
		}

		var memoize []byte

		var memoizeValue int64

		return &labelGenerator{
			AppendByte: func(b []byte, seriesID int) []byte {
				value := int64(seriesID / d)
				if memoize == nil || value != memoizeValue {
					memoizeValue = value
					memoize = memoize[:0]
					memoize = append(memoize, template[:i]...)
					//nolint:mnd // 10 is the base for decimal string conversion
					memoize = strconv.AppendInt(memoize, value, 10)
					memoize = append(memoize, template[i+end+1:]...)
				}

				return append(b, memoize...)
			},
		}, nil
	}

	return nil, errors.New("unsupported template")
}

type labelGenerator struct {
	AppendByte func([]byte, int) []byte
}

func newIdentityLabelGenerator(t string) *labelGenerator {
	return &labelGenerator{
		AppendByte: func(b []byte, _ int) []byte { return append(b, t...) },
	}
}

// this is opaque on purpose so that it can't be done anything to from the js side.
type labelTemplates struct {
	compiledTemplates []compiledTemplate
	labelValue        []byte
}
type compiledTemplate struct {
	name      string
	generator *labelGenerator
}

func compileLabelTemplates(labelsTemplate map[string]string) (*labelTemplates, error) {
	compiledTemplates := make([]compiledTemplate, len(labelsTemplate))
	{
		i := 0

		var err error

		for k, v := range labelsTemplate {
			compiledTemplates[i].name = k

			compiledTemplates[i].generator, err = compileTemplate(v)
			if err != nil {
				return nil, fmt.Errorf("error while compiling template %q, %w", v, err)
			}

			i++
		}
	}

	sort.Slice(compiledTemplates, func(i, j int) bool {
		return compiledTemplates[i].name < compiledTemplates[j].name
	})

	return &labelTemplates{
		compiledTemplates: compiledTemplates,
		//nolint:mnd // 128 bytes is a reasonable initial buffer size for label values
		labelValue: make([]byte, 128), // this is way more than necessary and it will grow if needed
	}, nil
}

// StoreFromTemplates generates and stores time series data using label templates.
func (c *Client) StoreFromTemplates(
	minValue, maxValue int,
	timestamp int64, minSeriesID, maxSeriesID int,
	labelsTemplate map[string]string,
) (httpext.Response, error) {
	template, err := compileLabelTemplates(labelsTemplate)
	if err != nil {
		return *httpext.NewResponse(), err
	}

	return c.StoreFromPrecompiledTemplates(minValue, maxValue, timestamp, minSeriesID, maxSeriesID, template)
}

func (template *labelTemplates) writeFor(w *bytes.Buffer, value float64, seriesID int, timestamp int64) {
	labelValue := template.labelValue[:] //nolint:gocritic // reuse slice to avoid allocations
	for _, template := range template.compiledTemplates {
		labelValue = labelValue[:0]

		//nolint:mnd // 0xa is protobuf wire format for length-delimited field
		w.WriteByte(0xa)

		labelValue = protowire.AppendVarint(labelValue, uint64(len(template.name)))
		n1 := len(labelValue)
		labelValue = template.generator.AppendByte(labelValue, seriesID)
		n2 := len(labelValue)
		labelValue = protowire.AppendVarint(labelValue, uint64(n2-n1)) // #nosec G115 -- len() result is always non-negative
		n3 := len(labelValue)

		// #nosec G115 -- len() result is always non-negative
		labelValue = protowire.AppendVarint(labelValue, uint64(n3+1+1+len(template.name)))
		w.Write(labelValue[n3:])
		//nolint:mnd // 0xa and 0x12 are protobuf wire format constants
		w.WriteByte(0xa)
		w.Write(labelValue[:n1])
		w.WriteString(template.name)
		//nolint:mnd // 0xa and 0x12 are protobuf wire format constants
		w.WriteByte(0x12)
		w.Write(labelValue[n2:n3])
		w.Write(labelValue[n1:n2])
	}

	labelValue = labelValue[:10]
	labelValue[0] = 0x9
	binary.LittleEndian.PutUint64(labelValue[1:9], math.Float64bits(value))
	labelValue[9] = 0x10
	// #nosec G115 -- timestamp is always positive milliseconds since Unix epoch
	labelValue = protowire.AppendVarint(labelValue, uint64(timestamp))

	n := len(labelValue)
	labelValue = labelValue[:n+1]
	labelValue[n] = 0x12
	labelValue = protowire.AppendVarint(labelValue, uint64(n))
	w.Write(labelValue[n:])
	w.Write(labelValue[:n])
	template.labelValue = labelValue

	// REVIEW TODO add error handling?
}

// StoreFromPrecompiledTemplates generates and stores time series data using precompiled label templates.
func (c *Client) StoreFromPrecompiledTemplates(
	minValue, maxValue int,
	timestamp int64, minSeriesID, maxSeriesID int,
	template *labelTemplates,
) (httpext.Response, error) {
	state := c.vu.State()
	if state == nil {
		return *httpext.NewResponse(), errors.New("State is nil")
	}

	// #nosec G404 -- This is test data generation for load testing, not cryptographic use
	r := rand.New(rand.NewSource(time.Now().Unix()))

	buf, err := generateFromPrecompiledTemplates(r, minValue, maxValue, timestamp, minSeriesID, maxSeriesID, template)
	if err != nil {
		return *httpext.NewResponse(), err
	}

	b := buf.Bytes()
	//nolint:mnd // 9 is a heuristic compression ratio (actual ratio is between 1/9 and 1/10)
	compressed := make([]byte, len(b)/9) // the general size is actually between 1/9 and 1/10th but this is closed enough
	compressed = snappy.Encode(compressed, b)

	res, err := c.send(state, compressed)
	if err != nil {
		return *httpext.NewResponse(), errors.Wrap(err, "remote-write request failed")
	}

	res.Request.Body = ""

	return res, nil
}

func (c *Client) store(batch []prompb.TimeSeries) (httpext.Response, error) {
	// Required for k6 metrics
	state := c.vu.State()
	if state == nil {
		return *httpext.NewResponse(), errors.New("State is nil")
	}

	req := prompb.WriteRequest{
		Timeseries: batch,
	}

	data, err := proto.Marshal(protoadapt.MessageV2Of(&req))
	if err != nil {
		return *httpext.NewResponse(), errors.Wrap(err, "failed to marshal remote-write request")
	}

	compressed := snappy.Encode(nil, data)

	res, err := c.send(state, compressed)
	if err != nil {
		return *httpext.NewResponse(), errors.Wrap(err, "remote-write request failed")
	}

	res.Request.Body = ""

	return res, nil
}

// send sends a batch of samples to the HTTP endpoint, the request is the proto marshalled
// and encoded bytes.
func (c *Client) send(state *lib.State, req []byte) (httpext.Response, error) {
	httpResp := httpext.NewResponse()

	r, err := http.NewRequestWithContext(c.vu.Context(), http.MethodPost, c.cfg.Url, nil)
	if err != nil {
		return *httpResp, err
	}

	for k, v := range c.cfg.Headers {
		r.Header.Set(k, v)

		if k == "Host" {
			r.Host = v
		}
	}

	// explicit config overwrites any previously set matching headers
	r.Header.Add("Content-Encoding", "snappy")
	r.Header.Set("Content-Type", "application/x-protobuf")
	r.Header.Set("User-Agent", c.cfg.UserAgent)
	r.Header.Set("X-Prometheus-Remote-Write-Version", "0.0.2")

	if c.cfg.TenantName != "" {
		r.Header.Set("X-Scope-Orgid", c.cfg.TenantName)
	}

	duration, err := str2duration.ParseDuration(c.cfg.Timeout)
	if err != nil {
		return *httpResp, err
	}

	u, err := url.Parse(c.cfg.Url)
	if err != nil {
		return *httpResp, err
	}

	url, _ := httpext.NewURL(c.cfg.Url, u.Host+u.Path)

	response, err := httpext.MakeRequest(c.vu.Context(), state, &httpext.ParsedHTTPRequest{
		URL:              &url,
		Req:              r,
		Body:             bytes.NewBuffer(req),
		Throw:            state.Options.Throw.Bool,
		Redirects:        state.Options.MaxRedirects,
		Timeout:          duration,
		ResponseCallback: ResponseCallback,
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	})
	if err != nil {
		return *httpResp, err
	}

	return *response, err
}

func generateFromPrecompiledTemplates(
	r *rand.Rand,
	minValue, maxValue int,
	timestamp int64, minSeriesID, maxSeriesID int,
	template *labelTemplates,
) (*bytes.Buffer, error) {
	//nolint:mnd // 1024 bytes is a reasonable initial buffer size
	bigB := make([]byte, 1024)
	buf := new(bytes.Buffer)
	buf.Reset()

	tsBuf := new(bytes.Buffer)
	bigB[0] = 0xa

	template.writeFor(tsBuf, valueBetween(r, minValue, maxValue), minSeriesID, timestamp)

	bigB = protowire.AppendVarint(bigB[:1], uint64(tsBuf.Len())) // #nosec G115 -- buffer Len() is always non-negative
	buf.Write(bigB)

	_, err := tsBuf.WriteTo(buf)
	if err != nil {
		return nil, err
	}

	//nolint:mnd // 2 is a heuristic padding factor for buffer growth
	buf.Grow((buf.Len() + 2) * (maxSeriesID - minSeriesID)) // heuristics to try to get big enough buffer in one go

	for seriesID := minSeriesID + 1; seriesID < maxSeriesID; seriesID++ {
		tsBuf.Reset()

		bigB[0] = 0xa

		template.writeFor(tsBuf, valueBetween(r, minValue, maxValue), seriesID, timestamp)

		bigB = protowire.AppendVarint(bigB[:1], uint64(tsBuf.Len())) // #nosec G115 -- buffer Len() is always non-negative
		buf.Write(bigB)

		_, err = tsBuf.WriteTo(buf)
		if err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func valueBetween(r *rand.Rand, minVal, maxVal int) float64 {
	return (r.Float64() * float64(maxVal-minVal)) + float64(minVal)
}
