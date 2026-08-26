package loki

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/loki/v3/pkg/logqlmodel/stats"
	"github.com/grafana/xk6-loki/flog"
	"github.com/prometheus/common/model"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

const (
	ContentTypeProtobuf   = "application/x-protobuf"
	ContentTypeJSON       = "application/json"
	ContentEncodingSnappy = "snappy"
	ContentEncodingGzip   = "gzip"

	TenantPrefix = "xk6-tenant"
)

type labelValues struct {
	name   model.LabelName
	values []string
}

type Client struct {
	vu      modules.VU
	client  *http.Client
	cfg     *Config
	metrics lokiMetrics
	rand    *rand.Rand
	faker   *gofakeit.Faker
	flog    *flog.Flog
	labels  []labelValues
}

type Config struct {
	URL           url.URL
	UserAgent     string
	Timeout       time.Duration
	TenantID      string
	Cardinalities map[string]int
	Labels        LabelPool
	ProtobufRatio float64
	RandSeed      int64
}

func (c *Client) InstantQuery(logQuery string, limit int) (httpext.Response, error) {
	return c.instantQuery(logQuery, limit, time.Now())
}

func (c *Client) InstantQueryAt(logQuery string, limit int, instant int64) (httpext.Response, error) {
	return c.instantQuery(logQuery, limit, time.Unix(instant, 0))
}

func (c *Client) instantQuery(logQuery string, limit int, now time.Time) (httpext.Response, error) {
	q := &Query{
		Type:        InstantQuery,
		QueryString: logQuery,
		Limit:       limit,
	}

	q.SetInstant(now)
	response, err := c.sendQuery(q)
	if err == nil && IsSuccessfulResponse(response.Status) {
		err = c.reportMetricsFromStats(response, InstantQuery)
	}
	return response, err
}

func (c *Client) RangeQuery(logQuery string, duration string, limit int) (httpext.Response, error) {
	return c.rangeQuery(logQuery, duration, limit, time.Now())
}

func (c *Client) RangeQueryAt(logQuery string, duration string, limit int, instant int64) (httpext.Response, error) {
	return c.rangeQuery(logQuery, duration, limit, time.Unix(instant, 0))
}

func (c *Client) rangeQuery(logQuery string, duration string, limit int, now time.Time) (httpext.Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return httpext.Response{}, err
	}
	q := &Query{
		Type:        RangeQuery,
		QueryString: logQuery,
		Start:       now.Add(-dur),
		End:         now,
		Limit:       limit,
	}
	response, err := c.sendQuery(q)
	if err == nil && IsSuccessfulResponse(response.Status) {
		err = c.reportMetricsFromStats(response, RangeQuery)
	}
	return response, err
}

func (c *Client) LabelsQuery(duration string) (httpext.Response, error) {
	return c.labelsQuery(duration, time.Now())
}

func (c *Client) LabelsQueryAt(duration string, instant int64) (httpext.Response, error) {
	return c.labelsQuery(duration, time.Unix(instant, 0))
}

func (c *Client) labelsQuery(duration string, now time.Time) (httpext.Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return httpext.Response{}, err
	}
	q := &Query{
		Type:  LabelsQuery,
		Start: now.Add(-dur),
		End:   now,
	}
	return c.sendQuery(q)
}

func (c *Client) LabelValuesQuery(label string, duration string) (httpext.Response, error) {
	return c.labelValuesQuery(label, duration, time.Now())
}

func (c *Client) LabelValuesQueryAt(label string, duration string, instant int64) (httpext.Response, error) {
	return c.labelValuesQuery(label, duration, time.Unix(instant, 0))
}

func (c *Client) labelValuesQuery(label string, duration string, now time.Time) (httpext.Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return httpext.Response{}, err
	}
	q := &Query{
		Type:       LabelValuesQuery,
		Start:      now.Add(-dur),
		End:        now,
		PathParams: []interface{}{label},
	}
	return c.sendQuery(q)
}

func (c *Client) SeriesQuery(matchers string, duration string) (httpext.Response, error) {
	return c.seriesQuery(matchers, duration, time.Now())
}

func (c *Client) SeriesQueryAt(matchers string, duration string, instant int64) (httpext.Response, error) {
	return c.seriesQuery(matchers, duration, time.Unix(instant, 0))
}

func (c *Client) seriesQuery(matchers string, duration string, now time.Time) (httpext.Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return httpext.Response{}, err
	}
	q := &Query{
		Type:        SeriesQuery,
		QueryString: matchers,
		Start:       now.Add(-dur),
		End:         now,
	}
	return c.sendQuery(q)
}

// buildURL concatinates a URL `http://foo/bar` with a path `/buzz` and a query string `?query=...`.
func buildURL(u, p, qs string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	url.Path = path.Join(url.Path, p)
	url.RawQuery = qs
	return url.String(), nil
}

func (c *Client) sendQuery(q *Query) (httpext.Response, error) {
	state := c.vu.State()
	if state == nil {
		return *httpext.NewResponse(), errors.New("state is nil")
	}

	httpResp := httpext.NewResponse()
	path := q.Endpoint()

	urlString, err := buildURL(c.cfg.URL.String(), path, q.Values().Encode())
	if err != nil {
		return *httpext.NewResponse(), err
	}

	r, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return *httpResp, err
	}

	r.Header.Set("User-Agent", c.cfg.UserAgent)
	r.Header.Set("Accept", ContentTypeJSON)
	if c.cfg.TenantID != "" {
		r.Header.Set("X-Scope-OrgID", c.cfg.TenantID)
	} else {
		r.Header.Set("X-Scope-OrgID", fmt.Sprintf("%s-%d", TenantPrefix, state.VUID))
	}

	url, _ := httpext.NewURL(urlString, path)
	response, err := httpext.MakeRequest(c.vu.Context(), state, &httpext.ParsedHTTPRequest{
		URL:              &url,
		Req:              r,
		Throw:            state.Options.Throw.Bool,
		Redirects:        state.Options.MaxRedirects,
		Timeout:          c.cfg.Timeout,
		ResponseCallback: IsSuccessfulResponse,
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	})
	if err != nil {
		return *httpResp, err
	}

	return *response, err
}

func (c *Client) Push() (httpext.Response, error) {
	// 5 streams per batch
	// batch size between 800KB and 1MB
	return c.PushParameterized(5, 800*1024, 1024*1024)
}

// PushParametrized is deprecated in favor or PushParameterized
func (c *Client) PushParametrized(streams, minBatchSize, maxBatchSize int) (httpext.Response, error) {
	if state := c.vu.State(); state == nil {
		return *httpext.NewResponse(), errors.New("state is nil")
	} else {
		state.Logger.Warn("method pushParametrized() is deprecated and will be removed in future releases; please use pushParameterized() instead")
	}
	return c.PushParameterized(streams, minBatchSize, maxBatchSize)
}

func (c *Client) PushParameterized(streams, minBatchSize, maxBatchSize int) (httpext.Response, error) {
	if minBatchSize > maxBatchSize {
		return *httpext.NewResponse(), errors.New("minimum batch size needs to be smaller or equal to max batch size")
	}
	state := c.vu.State()
	if state == nil {
		return *httpext.NewResponse(), errors.New("state is nil")
	}

	batch := c.newBatch(streams, minBatchSize, maxBatchSize)
	return c.pushBatch(batch)
}

func (c *Client) pushBatch(batch *Batch) (httpext.Response, error) {
	state := c.vu.State()
	if state == nil {
		return *httpext.NewResponse(), errors.New("state is nil")
	}

	var buf []byte
	var err error

	// Use snappy encoded Protobuf for 90% of the requests
	// Use JSON encoding for 10% of the requests
	encodeSnappy := c.rand.Float64() < c.cfg.ProtobufRatio
	if encodeSnappy {
		buf, _, err = batch.encodeSnappy()
	} else {
		buf, _, err = batch.encodeJSON()
	}
	if err != nil {
		return *httpext.NewResponse(), fmt.Errorf("failed to encode payload: %w", err)
	}

	res, err := c.send(state, buf, encodeSnappy)
	if err != nil {
		return *httpext.NewResponse(), fmt.Errorf("push request failed: %w", err)
	}
	res.Request.Body = ""
	if IsSuccessfulResponse(res.Status) {
		c.reportMetricsFromBatch(batch)
	}

	return res, err
}

func (c *Client) send(state *lib.State, buf []byte, useProtobuf bool) (httpext.Response, error) {
	httpResp := httpext.NewResponse()
	path := "/loki/api/v1/push"
	r, err := http.NewRequest(http.MethodPost, c.cfg.URL.String()+path, nil)
	if err != nil {
		return *httpResp, err
	}

	r.Header.Set("User-Agent", c.cfg.UserAgent)
	r.Header.Set("Accept", ContentTypeJSON)
	if c.cfg.TenantID != "" {
		r.Header.Set("X-Scope-OrgID", c.cfg.TenantID)
	} else {
		r.Header.Set("X-Scope-OrgID", fmt.Sprintf("%s-%d", TenantPrefix, state.VUID))
	}
	if useProtobuf {
		r.Header.Set("Content-Type", ContentTypeProtobuf)
		r.Header.Add("Content-Encoding", ContentEncodingSnappy)
	} else {
		r.Header.Set("Content-Type", ContentTypeJSON)
	}

	url, _ := httpext.NewURL(c.cfg.URL.String()+path, path)
	response, err := httpext.MakeRequest(c.vu.Context(), state, &httpext.ParsedHTTPRequest{
		URL:              &url,
		Req:              r,
		Body:             bytes.NewBuffer(buf),
		Throw:            state.Options.Throw.Bool,
		Redirects:        state.Options.MaxRedirects,
		Timeout:          c.cfg.Timeout,
		ResponseCallback: IsSuccessfulResponse,
		TagsAndMeta:      state.Tags.GetCurrentValues(),
	})
	if err != nil {
		return *httpResp, err
	}

	return *response, err
}

func IsSuccessfulResponse(n int) bool {
	// report all 2xx respones as successful requests
	return n/100 == 2
}

type responseWithStats struct {
	Data struct {
		Stats stats.Result
	}
}

func (c *Client) reportMetricsFromStats(response httpext.Response, queryType QueryType) error {
	responseBody, ok := response.Body.(string)
	if !ok {
		return errors.New("response body is not a string")
	}
	responseWithStats := responseWithStats{}
	err := json.Unmarshal([]byte(responseBody), &responseWithStats)
	if err != nil {
		return fmt.Errorf("error unmarshalling response body to response with stats: %w", err)
	}
	now := time.Now()
	ctm := c.vu.State().Tags.GetCurrentValues()
	tags := ctm.Tags.With("endpoint", queryType.Endpoint())
	ctx := c.vu.Context()
	metrics.PushIfNotDone(ctx, c.vu.State().Samples, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.BytesProcessedTotal,
					Tags:   tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(responseWithStats.Data.Stats.Summary.TotalBytesProcessed),
				Time:     now,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.BytesProcessedPerSeconds,
					Tags:   tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(responseWithStats.Data.Stats.Summary.BytesProcessedPerSecond),
				Time:     now,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.LinesProcessedTotal,
					Tags:   tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(responseWithStats.Data.Stats.Summary.TotalLinesProcessed),
				Time:     now,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.LinesProcessedPerSeconds,
					Tags:   tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(responseWithStats.Data.Stats.Summary.LinesProcessedPerSecond),
				Time:     now,
			},
		},
	})
	return nil
}

func (c *Client) reportMetricsFromBatch(batch *Batch) {
	lines := 0
	for _, stream := range batch.Streams {
		lines += len(stream.Entries)
	}

	now := time.Now()
	ctx := c.vu.Context()
	ctm := c.vu.State().Tags.GetCurrentValues()

	metrics.PushIfNotDone(ctx, c.vu.State().Samples, metrics.ConnectedSamples{
		Samples: []metrics.Sample{
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.ClientUncompressedBytes,
					Tags:   ctm.Tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(batch.Bytes),
				Time:     now,
			},
			{
				TimeSeries: metrics.TimeSeries{
					Metric: c.metrics.ClientLines,
					Tags:   ctm.Tags,
				},
				Metadata: ctm.Metadata,
				Value:    float64(lines),
				Time:     now,
			},
		},
	})
}
