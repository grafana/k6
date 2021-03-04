// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trace // import "go.opentelemetry.io/otel/sdk/trace"

import (
	"context"

	"go.opentelemetry.io/otel"
	export "go.opentelemetry.io/otel/sdk/export/trace"
)

// SimpleSpanProcessor is a SpanProcessor that synchronously sends all
// SpanSnapshots to a trace.Exporter when the span finishes.
type SimpleSpanProcessor struct {
	e export.SpanExporter
}

var _ SpanProcessor = (*SimpleSpanProcessor)(nil)

// NewSimpleSpanProcessor returns a new SimpleSpanProcessor that will
// synchronously send SpanSnapshots to the exporter.
func NewSimpleSpanProcessor(exporter export.SpanExporter) *SimpleSpanProcessor {
	ssp := &SimpleSpanProcessor{
		e: exporter,
	}
	return ssp
}

// OnStart method does nothing.
func (ssp *SimpleSpanProcessor) OnStart(parent context.Context, s ReadWriteSpan) {
}

// OnEnd method exports a ReadOnlySpan using the associated exporter.
func (ssp *SimpleSpanProcessor) OnEnd(s ReadOnlySpan) {
	if ssp.e != nil && s.SpanContext().IsSampled() {
		ss := s.Snapshot()
		if err := ssp.e.ExportSpans(context.Background(), []*export.SpanSnapshot{ss}); err != nil {
			otel.Handle(err)
		}
	}
}

// Shutdown method does nothing. There is no data to cleanup.
func (ssp *SimpleSpanProcessor) Shutdown(_ context.Context) error {
	return nil
}

// ForceFlush does nothing as there is no data to flush.
func (ssp *SimpleSpanProcessor) ForceFlush() {
}
