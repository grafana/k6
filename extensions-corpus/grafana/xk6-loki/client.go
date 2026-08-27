package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/loki/v3/pkg/logqlmodel/stats"
	"github.com/grafana/xk6-loki/flog"
	"github.com/prometheus/common/model"
	"go.k6.io/k6-extension-api"
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
	vu      extensionapi.VU
	cfg     *Config
	metrics lokiMetrics
	rand    *rand.Rand
	faker   *gofakeit.Faker
	flog    *flog.Flog
	labels  []labelValues
}

// Response is the portable JavaScript response returned by Loki operations.
type Response struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func emptyResponse() Response { return Response{Headers: make(map[string]string)} }

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

func (c *Client) InstantQuery(logQuery string, limit int) (Response, error) {
	return c.instantQuery(logQuery, limit, time.Now())
}

func (c *Client) InstantQueryAt(logQuery string, limit int, instant int64) (Response, error) {
	return c.instantQuery(logQuery, limit, time.Unix(instant, 0))
}

func (c *Client) instantQuery(logQuery string, limit int, now time.Time) (Response, error) {
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

func (c *Client) RangeQuery(logQuery string, duration string, limit int) (Response, error) {
	return c.rangeQuery(logQuery, duration, limit, time.Now())
}

func (c *Client) RangeQueryAt(logQuery string, duration string, limit int, instant int64) (Response, error) {
	return c.rangeQuery(logQuery, duration, limit, time.Unix(instant, 0))
}

func (c *Client) rangeQuery(logQuery string, duration string, limit int, now time.Time) (Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return emptyResponse(), err
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

func (c *Client) LabelsQuery(duration string) (Response, error) {
	return c.labelsQuery(duration, time.Now())
}

func (c *Client) LabelsQueryAt(duration string, instant int64) (Response, error) {
	return c.labelsQuery(duration, time.Unix(instant, 0))
}

func (c *Client) labelsQuery(duration string, now time.Time) (Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return emptyResponse(), err
	}
	q := &Query{
		Type:  LabelsQuery,
		Start: now.Add(-dur),
		End:   now,
	}
	return c.sendQuery(q)
}

func (c *Client) LabelValuesQuery(label string, duration string) (Response, error) {
	return c.labelValuesQuery(label, duration, time.Now())
}

func (c *Client) LabelValuesQueryAt(label string, duration string, instant int64) (Response, error) {
	return c.labelValuesQuery(label, duration, time.Unix(instant, 0))
}

func (c *Client) labelValuesQuery(label string, duration string, now time.Time) (Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return emptyResponse(), err
	}
	q := &Query{
		Type:       LabelValuesQuery,
		Start:      now.Add(-dur),
		End:        now,
		PathParams: []interface{}{label},
	}
	return c.sendQuery(q)
}

func (c *Client) SeriesQuery(matchers string, duration string) (Response, error) {
	return c.seriesQuery(matchers, duration, time.Now())
}

func (c *Client) SeriesQueryAt(matchers string, duration string, instant int64) (Response, error) {
	return c.seriesQuery(matchers, duration, time.Unix(instant, 0))
}

func (c *Client) seriesQuery(matchers string, duration string, now time.Time) (Response, error) {
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return emptyResponse(), err
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

func (c *Client) sendQuery(q *Query) (Response, error) {
	path := q.Endpoint()

	urlString, err := buildURL(c.cfg.URL.String(), path, q.Values().Encode())
	if err != nil {
		return emptyResponse(), err
	}

	r, err := http.NewRequest(http.MethodGet, urlString, nil)
	if err != nil {
		return emptyResponse(), err
	}

	r.Header.Set("User-Agent", c.cfg.UserAgent)
	r.Header.Set("Accept", ContentTypeJSON)
	if c.cfg.TenantID != "" {
		r.Header.Set("X-Scope-OrgID", c.cfg.TenantID)
	} else {
		id, err := c.vuID()
		if err != nil {
			return emptyResponse(), err
		}
		r.Header.Set("X-Scope-OrgID", fmt.Sprintf("%s-%d", TenantPrefix, id))
	}
	return c.do(r)
}

func (c *Client) Push() (Response, error) {
	// 5 streams per batch
	// batch size between 800KB and 1MB
	return c.PushParameterized(5, 800*1024, 1024*1024)
}

// PushParametrized is deprecated in favor or PushParameterized
func (c *Client) PushParametrized(streams, minBatchSize, maxBatchSize int) (Response, error) {
	return c.PushParameterized(streams, minBatchSize, maxBatchSize)
}

func (c *Client) PushParameterized(streams, minBatchSize, maxBatchSize int) (Response, error) {
	if minBatchSize > maxBatchSize {
		return emptyResponse(), errors.New("minimum batch size needs to be smaller or equal to max batch size")
	}

	batch := c.newBatch(streams, minBatchSize, maxBatchSize)
	return c.pushBatch(batch)
}

func (c *Client) pushBatch(batch *Batch) (Response, error) {

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
		return emptyResponse(), fmt.Errorf("failed to encode payload: %w", err)
	}

	res, err := c.send(buf, encodeSnappy)
	if err != nil {
		return emptyResponse(), fmt.Errorf("push request failed: %w", err)
	}
	if IsSuccessfulResponse(res.Status) {
		c.reportMetricsFromBatch(batch)
	}

	return res, err
}

func (c *Client) send(buf []byte, useProtobuf bool) (Response, error) {
	path := "/loki/api/v1/push"
	r, err := http.NewRequest(http.MethodPost, c.cfg.URL.String()+path, bytes.NewReader(buf))
	if err != nil {
		return emptyResponse(), err
	}

	r.Header.Set("User-Agent", c.cfg.UserAgent)
	r.Header.Set("Accept", ContentTypeJSON)
	if c.cfg.TenantID != "" {
		r.Header.Set("X-Scope-OrgID", c.cfg.TenantID)
	} else {
		id, err := c.vuID()
		if err != nil {
			return emptyResponse(), err
		}
		r.Header.Set("X-Scope-OrgID", fmt.Sprintf("%s-%d", TenantPrefix, id))
	}
	if useProtobuf {
		r.Header.Set("Content-Type", ContentTypeProtobuf)
		r.Header.Add("Content-Encoding", ContentEncodingSnappy)
	} else {
		r.Header.Set("Content-Type", ContentTypeJSON)
	}

	return c.do(r)
}

func (c *Client) vuID() (uint64, error) {
	identity, ok := c.vu.(extensionapi.VUIdentity)
	if !ok {
		return 0, errors.New("extension API VU identity capability is unavailable")
	}
	return identity.VUID(), nil
}

func (c *Client) do(request *http.Request) (Response, error) {
	httpClient, ok := c.vu.(extensionapi.HTTP)
	if !ok {
		return emptyResponse(), extensionapi.ErrHTTPUnavailable
	}
	ctx, cancel := context.WithTimeout(c.vu.Context(), c.cfg.Timeout)
	defer cancel()
	result, err := httpClient.Do(ctx, request.WithContext(ctx), extensionapi.HTTPOptions{})
	if err != nil {
		return emptyResponse(), err
	}
	defer result.Response.Body.Close() //nolint:errcheck // body is fully read below
	body, err := io.ReadAll(result.Response.Body)
	if err != nil {
		return emptyResponse(), err
	}
	headers := make(map[string]string, len(result.Response.Header))
	for key, values := range result.Response.Header {
		headers[key] = strings.Join(values, ",")
	}
	return Response{Status: result.Response.StatusCode, Headers: headers, Body: string(body)}, nil
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

func (c *Client) reportMetricsFromStats(response Response, queryType QueryType) error {
	responseWithStats := responseWithStats{}
	err := json.Unmarshal([]byte(response.Body), &responseWithStats)
	if err != nil {
		return fmt.Errorf("error unmarshalling response body to response with stats: %w", err)
	}
	now := time.Now()
	metrics, ok := c.vu.(extensionapi.Metrics)
	if !ok {
		return extensionapi.ErrMetricsUnavailable
	}
	tags := metrics.CurrentTags().With(map[string]string{"endpoint": queryType.Endpoint()})
	return metrics.Emit(c.vu.Context(), []extensionapi.Sample{
		{Metric: c.metrics.BytesProcessedTotal, Value: float64(responseWithStats.Data.Stats.Summary.TotalBytesProcessed), Time: now, Tags: tags},
		{Metric: c.metrics.BytesProcessedPerSeconds, Value: float64(responseWithStats.Data.Stats.Summary.BytesProcessedPerSecond), Time: now, Tags: tags},
		{Metric: c.metrics.LinesProcessedTotal, Value: float64(responseWithStats.Data.Stats.Summary.TotalLinesProcessed), Time: now, Tags: tags},
		{Metric: c.metrics.LinesProcessedPerSeconds, Value: float64(responseWithStats.Data.Stats.Summary.LinesProcessedPerSecond), Time: now, Tags: tags},
	})
}

func (c *Client) reportMetricsFromBatch(batch *Batch) {
	lines := 0
	for _, stream := range batch.Streams {
		lines += len(stream.Entries)
	}

	now := time.Now()
	metrics, ok := c.vu.(extensionapi.Metrics)
	if !ok {
		return
	}
	_ = metrics.Emit(c.vu.Context(), []extensionapi.Sample{
		{Metric: c.metrics.ClientUncompressedBytes, Value: float64(batch.Bytes), Time: now, Tags: metrics.CurrentTags()},
		{Metric: c.metrics.ClientLines, Value: float64(lines), Time: now, Tags: metrics.CurrentTags()},
	})
}
