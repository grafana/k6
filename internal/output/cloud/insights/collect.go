package insights

import (
	"strconv"
	"sync"

	"go.k6.io/k6/internal/cloudapi/insights"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
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

		traceID, found := trail.Metadata[metadataTraceIDKey]
		if !found {
			continue
		}

		m := insights.RequestMetadata{
			TraceID: traceID,
			Start:   trail.EndTime.Add(-trail.Duration),
			End:     trail.EndTime,
			TestRunLabels: insights.TestRunLabels{
				ID:       c.testRunID,
				Scenario: c.getStringTagFromTrail(trail, scenarioTag),
				Group:    c.getStringTagFromTrail(trail, groupTag),
			},
			ProtocolLabels: insights.ProtocolHTTPLabels{
				URL:        c.getStringTagFromTrail(trail, nameTag),
				Method:     c.getStringTagFromTrail(trail, methodTag),
				StatusCode: c.getIntTagFromTrail(trail, statusTag),
			},
		}

		newBuffer = append(newBuffer, m)
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

func (c *Collector) getStringTagFromTrail(trail *httpext.Trail, key string) string {
	if tag, found := trail.Tags.Get(key); found {
		return tag
	}

	return ""
}

func (c *Collector) getIntTagFromTrail(trail *httpext.Trail, key string) int64 {
	if tag, found := trail.Tags.Get(key); found {
		tagInt, err := strconv.ParseInt(tag, 10, 64)
		if err != nil {
			return 0
		}

		return tagInt
	}

	return 0
}
