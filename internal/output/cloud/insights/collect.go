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
	methodTag          = "method"
	statusTag          = "status"
)

// RequestMetadatasCollector is an interface for collecting request metadatas
// and retrieving them, so they can be flushed using a flusher.
type RequestMetadatasCollector interface {
	CollectRequestMetadatas([]metrics.SampleContainer)
	PopAll() insights.RequestMetadatas
}

// Collector is an implementation of RequestMetadatasCollector.
// Its purpose is to filter and store httpext.Trail samples
// containing tracing data for later flushing.
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

// CollectRequestMetadatas filters httpext.Trail samples containing trace ids and stores them as
// insights.RequestMetadatas in the buffer.
func (c *Collector) CollectRequestMetadatas(sampleContainers []metrics.SampleContainer) {
	if len(sampleContainers) < 1 {
		return
	}

	// TODO(lukasz, other-proto-support): Support grpc/websocket trails.
	var newBuffer insights.RequestMetadatas
	for _, sampleContainer := range sampleContainers {
		trail, ok := sampleContainer.(*httpext.Trail)
		if !ok {
			continue
		}

		if m, ok := c.requestMetadataFromHTTPTrail(trail); ok {
			newBuffer = append(newBuffer, m)
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
