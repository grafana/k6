package insights

import (
	"strconv"
	"sync"

	"go.k6.io/k6/v2/internal/cloudapi/insights"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

const (
	metadataTraceIDKey = "trace_id"
	scenarioTag        = "scenario"
	groupTag           = "group"
	nameTag            = "name"
	urlTag             = "url"
	methodTag          = "method"
	serviceTag         = "service"
	statusTag          = "status"
)

// RequestMetadatasCollector is an interface for collecting request metadatas
// and retrieving them, so they can be flushed using a flusher.
type RequestMetadatasCollector interface {
	CollectRequestMetadatas([]metrics.SampleContainer)
	PopAll() insights.RequestMetadatas
}

// Collector is an implementation of RequestMetadatasCollector.
// Its purpose is to filter and store httpext.Trail samples and gRPC request
// samples containing tracing data for later flushing.
type Collector struct {
	testRunID int64
	buffer    insights.RequestMetadatas
	bufferMu  *sync.Mutex
}

// NewCollector creates a new Collector.
func NewCollector(testRunID int64) *Collector {
	return &Collector{
		testRunID: testRunID,
		buffer:    nil,
		bufferMu:  &sync.Mutex{},
	}
}

// CollectRequestMetadatas filters httpext.Trail and gRPC request duration samples
// containing trace ids and stores them as insights.RequestMetadatas in the buffer.
func (c *Collector) CollectRequestMetadatas(sampleContainers []metrics.SampleContainer) {
	if len(sampleContainers) < 1 {
		return
	}

	// TODO(other-proto-support): Support websocket trails.
	var newBuffer insights.RequestMetadatas
	for _, sampleContainer := range sampleContainers {
		if trail, ok := sampleContainer.(*httpext.Trail); ok {
			if m, ok := c.requestMetadataFromHTTPTrail(trail); ok {
				newBuffer = append(newBuffer, m)
			}

			continue
		}

		for _, sample := range sampleContainer.GetSamples() {
			if m, ok := c.requestMetadataFromGRPCSample(sample); ok {
				newBuffer = append(newBuffer, m)
			}
		}
	}

	if len(newBuffer) < 1 {
		return
	}

	c.bufferMu.Lock()
	defer c.bufferMu.Unlock()

	c.buffer = append(c.buffer, newBuffer...)
}

// PopAll returns all collected insights.RequestMetadatas and clears the buffer.
func (c *Collector) PopAll() insights.RequestMetadatas {
	c.bufferMu.Lock()
	defer c.bufferMu.Unlock()

	b := c.buffer
	c.buffer = nil
	return b
}

func (c *Collector) requestMetadataFromHTTPTrail(trail *httpext.Trail) (insights.RequestMetadata, bool) {
	traceID, found := trail.Metadata[metadataTraceIDKey]
	if !found {
		return insights.RequestMetadata{}, false
	}

	return insights.RequestMetadata{
		TraceID: traceID,
		Start:   trail.EndTime.Add(-trail.Duration),
		End:     trail.EndTime,
		TestRunLabels: insights.TestRunLabels{
			ID:       c.testRunID,
			Scenario: getStringTag(trail.Tags, scenarioTag),
			Group:    getStringTag(trail.Tags, groupTag),
		},
		ProtocolLabels: insights.ProtocolHTTPLabels{
			URL:        getStringTag(trail.Tags, nameTag),
			Method:     getStringTag(trail.Tags, methodTag),
			StatusCode: getIntTag(trail.Tags, statusTag),
		},
	}, true
}

// requestMetadataFromGRPCSample builds the request metadata of a gRPC request.
// gRPC requests are not reported as trails: the gRPC stats handler emits a
// single grpc_req_duration sample per RPC, carrying the request tags and
// metadata, so that is the sample to look for.
func (c *Collector) requestMetadataFromGRPCSample(sample metrics.Sample) (insights.RequestMetadata, bool) {
	if sample.Metric == nil || sample.Metric.Name != metrics.GRPCReqDurationName {
		return insights.RequestMetadata{}, false
	}

	traceID, found := sample.Metadata[metadataTraceIDKey]
	if !found {
		return insights.RequestMetadata{}, false
	}

	return insights.RequestMetadata{
		TraceID: traceID,
		Start:   sample.Time.Add(-metrics.ToD(sample.Value)),
		End:     sample.Time,
		TestRunLabels: insights.TestRunLabels{
			ID:       c.testRunID,
			Scenario: getStringTag(sample.Tags, scenarioTag),
			Group:    getStringTag(sample.Tags, groupTag),
		},
		ProtocolLabels: insights.ProtocolGRPCLabels{
			URL:        getStringTag(sample.Tags, urlTag),
			Service:    getStringTag(sample.Tags, serviceTag),
			Method:     getStringTag(sample.Tags, methodTag),
			StatusCode: getIntTag(sample.Tags, statusTag),
		},
	}, true
}

func getStringTag(tags *metrics.TagSet, key string) string {
	if tags == nil {
		return ""
	}

	if tag, found := tags.Get(key); found {
		return tag
	}

	return ""
}

func getIntTag(tags *metrics.TagSet, key string) int64 {
	tag := getStringTag(tags, key)
	if tag == "" {
		return 0
	}

	tagInt, err := strconv.ParseInt(tag, 10, 64)
	if err != nil {
		return 0
	}

	return tagInt
}
