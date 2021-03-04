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
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"

	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/internal"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	errorTypeKey    = label.Key("error.type")
	errorMessageKey = label.Key("error.message")
	errorEventName  = "error"
)

// ReadOnlySpan allows reading information from the data structure underlying a
// trace.Span. It is used in places where reading information from a span is
// necessary but changing the span isn't necessary or allowed.
// TODO: Should we make the methods unexported? The purpose of this interface
// is controlling access to `span` fields, not having multiple implementations.
type ReadOnlySpan interface {
	Name() string
	SpanContext() trace.SpanContext
	Parent() trace.SpanContext
	SpanKind() trace.SpanKind
	StartTime() time.Time
	EndTime() time.Time
	Attributes() []label.KeyValue
	Links() []trace.Link
	Events() []trace.Event
	StatusCode() codes.Code
	StatusMessage() string
	Tracer() trace.Tracer
	IsRecording() bool
	InstrumentationLibrary() instrumentation.Library
	Resource() *resource.Resource
	Snapshot() *export.SpanSnapshot
}

// ReadWriteSpan exposes the same methods as trace.Span and in addition allows
// reading information from the underlying data structure.
// This interface exposes the union of the methods of trace.Span (which is a
// "write-only" span) and ReadOnlySpan. New methods for writing or reading span
// information should be added under trace.Span or ReadOnlySpan, respectively.
type ReadWriteSpan interface {
	trace.Span
	ReadOnlySpan
}

var emptySpanContext = trace.SpanContext{}

// span is an implementation of the OpenTelemetry Span API representing the
// individual component of a trace.
type span struct {
	// mu protects the contents of this span.
	mu sync.Mutex

	// parent holds the parent span of this span as a trace.SpanContext.
	parent trace.SpanContext

	// spanKind represents the kind of this span as a trace.SpanKind.
	spanKind trace.SpanKind

	// name is the name of this span.
	name string

	// startTime is the time at which this span was started.
	startTime time.Time

	// endTime is the time at which this span was ended. It contains the zero
	// value of time.Time until the span is ended.
	endTime time.Time

	// statusCode represents the status of this span as a codes.Code value.
	statusCode codes.Code

	// statusMessage represents the status of this span as a string.
	statusMessage string

	// hasRemoteParent is true when this span has a remote parent span.
	hasRemoteParent bool

	// childSpanCount holds the number of child spans created for this span.
	childSpanCount int

	// resource contains attributes representing an entity that produced this
	// span.
	resource *resource.Resource

	// instrumentationLibrary defines the instrumentation library used to
	// provide instrumentation.
	instrumentationLibrary instrumentation.Library

	// spanContext holds the SpanContext of this span.
	spanContext trace.SpanContext

	// attributes are capped at configured limit. When the capacity is reached
	// an oldest entry is removed to create room for a new entry.
	attributes *attributesMap

	// messageEvents are stored in FIFO queue capped by configured limit.
	messageEvents *evictedQueue

	// links are stored in FIFO queue capped by configured limit.
	links *evictedQueue

	// executionTracerTaskEnd ends the execution tracer span.
	executionTracerTaskEnd func()

	// tracer is the SDK tracer that created this span.
	tracer *tracer
}

var _ trace.Span = &span{}

func (s *span) SpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}
	return s.spanContext
}

func (s *span) IsRecording() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endTime.IsZero()
}

func (s *span) SetStatus(code codes.Code, msg string) {
	if s == nil {
		return
	}
	if !s.IsRecording() {
		return
	}
	s.mu.Lock()
	s.statusCode = code
	s.statusMessage = msg
	s.mu.Unlock()
}

func (s *span) SetAttributes(attributes ...label.KeyValue) {
	if !s.IsRecording() {
		return
	}
	s.copyToCappedAttributes(attributes...)
}

// End ends the span.
//
// The only SpanOption currently supported is WithTimestamp which will set the
// end time for a Span's life-cycle.
//
// If this method is called while panicking an error event is added to the
// Span before ending it and the panic is continued.
func (s *span) End(options ...trace.SpanOption) {
	if s == nil {
		return
	}

	// Store the end time as soon as possible to avoid artificially increasing
	// the span's duration in case some operation below takes a while.
	et := internal.MonotonicEndTime(s.startTime)

	if recovered := recover(); recovered != nil {
		// Record but don't stop the panic.
		defer panic(recovered)
		s.addEvent(
			errorEventName,
			trace.WithAttributes(
				errorTypeKey.String(typeStr(recovered)),
				errorMessageKey.String(fmt.Sprint(recovered)),
			),
		)
	}

	if s.executionTracerTaskEnd != nil {
		s.executionTracerTaskEnd()
	}

	if !s.IsRecording() {
		return
	}

	config := trace.NewSpanConfig(options...)

	s.mu.Lock()
	if config.Timestamp.IsZero() {
		s.endTime = et
	} else {
		s.endTime = config.Timestamp
	}
	s.mu.Unlock()

	sps, ok := s.tracer.provider.spanProcessors.Load().(spanProcessorStates)
	mustExportOrProcess := ok && len(sps) > 0
	if mustExportOrProcess {
		for _, sp := range sps {
			sp.sp.OnEnd(s)
		}
	}
}

func (s *span) RecordError(err error, opts ...trace.EventOption) {
	if s == nil || err == nil || !s.IsRecording() {
		return
	}

	s.SetStatus(codes.Error, "")
	opts = append(opts, trace.WithAttributes(
		errorTypeKey.String(typeStr(err)),
		errorMessageKey.String(err.Error()),
	))
	s.addEvent(errorEventName, opts...)
}

func typeStr(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.PkgPath() == "" && t.Name() == "" {
		// Likely a builtin type.
		return t.String()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

func (s *span) Tracer() trace.Tracer {
	return s.tracer
}

func (s *span) AddEvent(name string, o ...trace.EventOption) {
	if !s.IsRecording() {
		return
	}
	s.addEvent(name, o...)
}

func (s *span) addEvent(name string, o ...trace.EventOption) {
	c := trace.NewEventConfig(o...)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageEvents.add(trace.Event{
		Name:       name,
		Attributes: c.Attributes,
		Time:       c.Timestamp,
	})
}

func (s *span) SetName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.name = name
	// SAMPLING
	noParent := !s.parent.SpanID.IsValid()
	var ctx trace.SpanContext
	if noParent {
		ctx = trace.SpanContext{}
	} else {
		// FIXME: Where do we get the parent context from?
		ctx = s.spanContext
	}
	data := samplingData{
		noParent:     noParent,
		remoteParent: s.hasRemoteParent,
		parent:       ctx,
		name:         name,
		cfg:          s.tracer.provider.config.Load().(*Config),
		span:         s,
		attributes:   s.attributes.toKeyValue(),
		links:        s.interfaceArrayToLinksArray(),
		kind:         s.spanKind,
	}
	sampled := makeSamplingDecision(data)

	// Adding attributes directly rather than using s.SetAttributes()
	// as s.mu is already locked and attempting to do so would deadlock.
	for _, a := range sampled.Attributes {
		s.attributes.add(a)
	}
}

func (s *span) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

func (s *span) Parent() trace.SpanContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parent
}

func (s *span) SpanKind() trace.SpanKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spanKind
}

func (s *span) StartTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTime
}

func (s *span) EndTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endTime
}

func (s *span) Attributes() []label.KeyValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attributes.evictList.Len() == 0 {
		return []label.KeyValue{}
	}
	return s.attributes.toKeyValue()
}

func (s *span) Links() []trace.Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.links.queue) == 0 {
		return []trace.Link{}
	}
	return s.interfaceArrayToLinksArray()
}

func (s *span) Events() []trace.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messageEvents.queue) == 0 {
		return []trace.Event{}
	}
	return s.interfaceArrayToMessageEventArray()
}

func (s *span) StatusCode() codes.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusCode
}

func (s *span) StatusMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusMessage
}

func (s *span) InstrumentationLibrary() instrumentation.Library {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instrumentationLibrary
}

func (s *span) Resource() *resource.Resource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resource
}

func (s *span) addLink(link trace.Link) {
	if !s.IsRecording() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links.add(link)
}

// Snapshot creates a snapshot representing the current state of the span as an
// export.SpanSnapshot and returns a pointer to it.
func (s *span) Snapshot() *export.SpanSnapshot {
	var sd export.SpanSnapshot
	s.mu.Lock()
	defer s.mu.Unlock()

	sd.ChildSpanCount = s.childSpanCount
	sd.EndTime = s.endTime
	sd.HasRemoteParent = s.hasRemoteParent
	sd.InstrumentationLibrary = s.instrumentationLibrary
	sd.Name = s.name
	sd.ParentSpanID = s.parent.SpanID
	sd.Resource = s.resource
	sd.SpanContext = s.spanContext
	sd.SpanKind = s.spanKind
	sd.StartTime = s.startTime
	sd.StatusCode = s.statusCode
	sd.StatusMessage = s.statusMessage

	if s.attributes.evictList.Len() > 0 {
		sd.Attributes = s.attributes.toKeyValue()
		sd.DroppedAttributeCount = s.attributes.droppedCount
	}
	if len(s.messageEvents.queue) > 0 {
		sd.MessageEvents = s.interfaceArrayToMessageEventArray()
		sd.DroppedMessageEventCount = s.messageEvents.droppedCount
	}
	if len(s.links.queue) > 0 {
		sd.Links = s.interfaceArrayToLinksArray()
		sd.DroppedLinkCount = s.links.droppedCount
	}
	return &sd
}

func (s *span) interfaceArrayToLinksArray() []trace.Link {
	linkArr := make([]trace.Link, 0)
	for _, value := range s.links.queue {
		linkArr = append(linkArr, value.(trace.Link))
	}
	return linkArr
}

func (s *span) interfaceArrayToMessageEventArray() []trace.Event {
	messageEventArr := make([]trace.Event, 0)
	for _, value := range s.messageEvents.queue {
		messageEventArr = append(messageEventArr, value.(trace.Event))
	}
	return messageEventArr
}

func (s *span) copyToCappedAttributes(attributes ...label.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range attributes {
		if a.Value.Type() != label.INVALID {
			s.attributes.add(a)
		}
	}
}

func (s *span) addChild() {
	if !s.IsRecording() {
		return
	}
	s.mu.Lock()
	s.childSpanCount++
	s.mu.Unlock()
}

func startSpanInternal(ctx context.Context, tr *tracer, name string, parent trace.SpanContext, remoteParent bool, o *trace.SpanConfig) *span {
	span := &span{}
	span.spanContext = parent

	cfg := tr.provider.config.Load().(*Config)

	if hasEmptySpanContext(parent) {
		// Generate both TraceID and SpanID
		span.spanContext.TraceID, span.spanContext.SpanID = cfg.IDGenerator.NewIDs(ctx)
	} else {
		// TraceID already exists, just generate a SpanID
		span.spanContext.SpanID = cfg.IDGenerator.NewSpanID(ctx, parent.TraceID)
	}

	span.attributes = newAttributesMap(cfg.MaxAttributesPerSpan)
	span.messageEvents = newEvictedQueue(cfg.MaxEventsPerSpan)
	span.links = newEvictedQueue(cfg.MaxLinksPerSpan)

	data := samplingData{
		noParent:     hasEmptySpanContext(parent),
		remoteParent: remoteParent,
		parent:       parent,
		name:         name,
		cfg:          cfg,
		span:         span,
		attributes:   o.Attributes,
		links:        o.Links,
		kind:         o.SpanKind,
	}
	sampled := makeSamplingDecision(data)

	if !span.spanContext.IsSampled() && !o.Record {
		return span
	}

	startTime := o.Timestamp
	if startTime.IsZero() {
		startTime = time.Now()
	}
	span.startTime = startTime

	span.spanKind = trace.ValidateSpanKind(o.SpanKind)
	span.name = name
	span.hasRemoteParent = remoteParent
	span.resource = cfg.Resource
	span.instrumentationLibrary = tr.instrumentationLibrary

	span.SetAttributes(sampled.Attributes...)

	span.parent = parent

	return span
}

func hasEmptySpanContext(parent trace.SpanContext) bool {
	return parent.SpanID == emptySpanContext.SpanID &&
		parent.TraceID == emptySpanContext.TraceID &&
		parent.TraceFlags == emptySpanContext.TraceFlags &&
		parent.TraceState.IsEmpty()
}

type samplingData struct {
	noParent     bool
	remoteParent bool
	parent       trace.SpanContext
	name         string
	cfg          *Config
	span         *span
	attributes   []label.KeyValue
	links        []trace.Link
	kind         trace.SpanKind
}

func makeSamplingDecision(data samplingData) SamplingResult {
	sampler := data.cfg.DefaultSampler
	spanContext := &data.span.spanContext
	sampled := sampler.ShouldSample(SamplingParameters{
		ParentContext:   data.parent,
		TraceID:         spanContext.TraceID,
		Name:            data.name,
		HasRemoteParent: data.remoteParent,
		Kind:            data.kind,
		Attributes:      data.attributes,
		Links:           data.links,
	})
	if sampled.Decision == RecordAndSample {
		spanContext.TraceFlags |= trace.FlagsSampled
	} else {
		spanContext.TraceFlags &^= trace.FlagsSampled
	}
	return sampled
}
