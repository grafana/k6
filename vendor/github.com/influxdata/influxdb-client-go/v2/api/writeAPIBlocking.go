// Copyright 2020-2021 InfluxData, Inc. All rights reserved.
// Use of this source code is governed by MIT
// license that can be found in the LICENSE file.

package api

import (
	"context"
	"strings"

	http2 "github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	iwrite "github.com/influxdata/influxdb-client-go/v2/internal/write"
)

// WriteAPIBlocking offers blocking methods for writing time series data synchronously into an InfluxDB server.
// It doesn't implicitly create batches of points. It is intended to use for writing less frequent data, such as a weather sensing, or if there is a need to have explicit control of failed batches.
//
// WriteAPIBlocking can be used concurrently.
// When using multiple goroutines for writing, use a single WriteAPIBlocking instance in all goroutines.
//
// To add implicit batching, use a wrapper, such as:
//	type writer struct {
//		batch []*write.Point
//		writeAPI api.WriteAPIBlocking
//		batchSize int
//	}
//
//	func (w *writer) CurrentBatch() []*write.Point {
//		return w.batch
//	}
//
//	func newWriter(writeAPI api.WriteAPIBlocking, batchSize int) *writer {
//		return &writer{
//			batch:     make([]*write.Point, 0, batchSize),
//			writeAPI:  writeAPI,
//			batchSize: batchSize,
//		}
//	}
//
//	func (w *writer) write(ctx context.Context, p *write.Point) error {
//		w.batch = append(w.batch, p)
//		if len(w.batch) == w.batchSize {
//			err := w.writeAPI.WritePoint(ctx, w.batch...)
//			if err != nil {
//				return err
//			}
//			w.batch = w.batch[:0]
//		}
//		return nil
//	}
type WriteAPIBlocking interface {
	// WriteRecord writes line protocol record(s) into bucket.
	// WriteRecord writes without implicit batching. Batch is created from given number of records
	// Non-blocking alternative is available in the WriteAPI interface
	WriteRecord(ctx context.Context, line ...string) error
	// WritePoint data point into bucket.
	// WritePoint writes without implicit batching. Batch is created from given number of points
	// Non-blocking alternative is available in the WriteAPI interface
	WritePoint(ctx context.Context, point ...*write.Point) error
}

// writeAPIBlocking implements WriteAPIBlocking interface
type writeAPIBlocking struct {
	service      *iwrite.Service
	writeOptions *write.Options
}

// NewWriteAPIBlocking creates new instance of blocking write client for writing data to bucket belonging to org
func NewWriteAPIBlocking(org string, bucket string, service http2.Service, writeOptions *write.Options) WriteAPIBlocking {
	return &writeAPIBlocking{service: iwrite.NewService(org, bucket, service, writeOptions), writeOptions: writeOptions}
}

func (w *writeAPIBlocking) write(ctx context.Context, line string) error {
	err := w.service.WriteBatch(ctx, iwrite.NewBatch(line, w.writeOptions.RetryInterval(), w.writeOptions.MaxRetryTime()))
	if err != nil {
		return err.Unwrap()
	}
	return nil
}

func (w *writeAPIBlocking) WriteRecord(ctx context.Context, line ...string) error {
	if len(line) > 0 {
		var sb strings.Builder
		for _, line := range line {
			b := []byte(line)
			b = append(b, 0xa)
			if _, err := sb.Write(b); err != nil {
				return err
			}
		}
		return w.write(ctx, sb.String())
	}
	return nil
}

func (w *writeAPIBlocking) WritePoint(ctx context.Context, point ...*write.Point) error {
	line, err := w.service.EncodePoints(point...)
	if err != nil {
		return err
	}
	return w.write(ctx, line)
}
