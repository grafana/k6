package insights

import (
	"context"

	"go.k6.io/k6/internal/cloudapi/insights"
)

// Client is an interface for sending request metadatas to the Insights API.
type Client interface {
	IngestRequestMetadatasBatch(context.Context, insights.RequestMetadatas) error
	Close() error
}

// RequestMetadatasFlusher is an interface for flushing data to the cloud.
type RequestMetadatasFlusher interface {
	Flush() error
}

// Flusher is an implementation of RequestMetadatasFlusher.
// Its purpose is to retrieve data from a collector
// and send it to the insights backend.
type Flusher struct {
	client    Client
	collector RequestMetadatasCollector
}

// NewFlusher creates a new Flusher.
func NewFlusher(client Client, collector RequestMetadatasCollector) *Flusher {
	return &Flusher{
		client:    client,
		collector: collector,
	}
}

// Flush retrieves data from the collector and sends it to the insights backend.
func (f *Flusher) Flush() error {
	requestMetadatas := f.collector.PopAll()
	if len(requestMetadatas) < 1 {
		return nil
	}

	return f.client.IngestRequestMetadatasBatch(context.Background(), requestMetadatas)
}
