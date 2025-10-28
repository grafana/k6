// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace // import "go.opentelemetry.io/otel/sdk/trace"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk"
	"go.opentelemetry.io/otel/sdk/internal/env"
	"go.opentelemetry.io/otel/sdk/trace/internal/x"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/semconv/v1.37.0/otelconv"
	"go.opentelemetry.io/otel/trace"
)

// Defaults for BatchSpanProcessorOptions.
const (
	DefaultMaxQueueSize = 2048
	// DefaultScheduleDelay is the delay interval between two consecutive exports, in milliseconds.
	DefaultScheduleDelay = 5000
	// DefaultExportTimeout is the duration after which an export is cancelled, in milliseconds.
	DefaultExportTimeout      = 30000
	DefaultMaxExportBatchSize = 512
)

var queueFull = otelconv.ErrorTypeAttr("queue_full")

// BatchSpanProcessorOption configures a BatchSpanProcessor.
type BatchSpanProcessorOption func(o *BatchSpanProcessorOptions)

// BatchSpanProcessorOptions is configuration settings for a
// BatchSpanProcessor.
type BatchSpanProcessorOptions struct {
	// MaxQueueSize is the maximum queue size to buffer spans for delayed processing. If the
	// queue gets full it drops the spans. Use BlockOnQueueFull to change this behavior.
	// The default value of MaxQueueSize is 2048.
	MaxQueueSize int

	// BatchTimeout is the maximum duration for constructing a batch. Processor
	// forcefully sends available spans when timeout is reached.
	// The default value of BatchTimeout is 5000 msec.
	BatchTimeout time.Duration

	// ExportTimeout specifies the maximum duration for exporting spans. If the timeout
	// is reached, the export will be cancelled.
	// The default value of ExportTimeout is 30000 msec.
	ExportTimeout time.Duration

	// MaxExportBatchSize is the maximum number of spans to process in a single batch.
	// If there are more than one batch worth of spans then it processes multiple batches
	// of spans one batch after the other without any delay.
	// The default value of MaxExportBatchSize is 512.
	MaxExportBatchSize int

	// BlockOnQueueFull blocks onEnd() and onStart() method if the queue is full
	// AND if BlockOnQueueFull is set to true.
	// Blocking option should be used carefully as it can severely affect the performance of an
	// application.
	BlockOnQueueFull bool
}

// batchSpanProcessor is a SpanProcessor that batches asynchronously-received
// spans and sends them to a trace.Exporter when complete.
type batchSpanProcessor struct {
	e SpanExporter
	o BatchSpanProcessorOptions

	queue   chan ReadOnlySpan
	dropped uint32

	selfObservabilityEnabled bool
	callbackRegistration     metric.Registration
	spansProcessedCounter    otelconv.SDKProcessorSpanProcessed
	componentNameAttr        attribute.KeyValue

	batch      []ReadOnlySpan
	batchMutex sync.Mutex
	timer      *time.Timer
	stopWait   sync.WaitGroup
	stopOnce   sync.Once
	stopCh     chan struct{}
	stopped    atomic.Bool
}

var _ SpanProcessor = (*batchSpanProcessor)(nil)

// NewBatchSpanProcessor creates a new SpanProcessor that will send completed
// span batches to the exporter with the supplied options.
//
// If the exporter is nil, the span processor will perform no action.
func NewBatchSpanProcessor(exporter SpanExporter, options ...BatchSpanProcessorOption) SpanProcessor {
	maxQueueSize := env.BatchSpanProcessorMaxQueueSize(DefaultMaxQueueSize)
	maxExportBatchSize := env.BatchSpanProcessorMaxExportBatchSize(DefaultMaxExportBatchSize)

	if maxExportBatchSize > maxQueueSize {
		maxExportBatchSize = min(DefaultMaxExportBatchSize, maxQueueSize)
	}

	o := BatchSpanProcessorOptions{
		BatchTimeout:       time.Duration(env.BatchSpanProcessorScheduleDelay(DefaultScheduleDelay)) * time.Millisecond,
		ExportTimeout:      time.Duration(env.BatchSpanProcessorExportTimeout(DefaultExportTimeout)) * time.Millisecond,
		MaxQueueSize:       maxQueueSize,
		MaxExportBatchSize: maxExportBatchSize,
	}
	for _, opt := range options {
		opt(&o)
	}
	bsp := &batchSpanProcessor{
		e:      exporter,
		o:      o,
		batch:  make([]ReadOnlySpan, 0, o.MaxExportBatchSize),
		timer:  time.NewTimer(o.BatchTimeout),
		queue:  make(chan ReadOnlySpan, o.MaxQueueSize),
		stopCh: make(chan struct{}),
	}

	if x.SelfObservability.Enabled() {
		bsp.selfObservabilityEnabled = true
		bsp.componentNameAttr = componentName()

		var err error
		bsp.spansProcessedCounter, bsp.callbackRegistration, err = newBSPObs(
			bsp.componentNameAttr,
			func() int64 { return int64(len(bsp.queue)) },
			int64(bsp.o.MaxQueueSize),
		)
		if err != nil {
			otel.Handle(err)
		}
	}

	bsp.stopWait.Add(1)
	go func() {
		defer bsp.stopWait.Done()
		bsp.processQueue()
		bsp.drainQueue()
	}()

	return bsp
}

var processorIDCounter atomic.Int64

// nextProcessorID returns an identifier for this batch span processor,
// starting with 0 and incrementing by 1 each time it is called.
func nextProcessorID() int64 {
	return processorIDCounter.Add(1) - 1
}

func componentName() attribute.KeyValue {
	id := nextProcessorID()
	name := fmt.Sprintf("%s/%d", otelconv.ComponentTypeBatchingSpanProcessor, id)
	return semconv.OTelComponentName(name)
}

// newBSPObs creates and returns a new set of metrics instruments and a
// registration for a BatchSpanProcessor. It is the caller's responsibility
// to unregister the registration when it is no longer needed.
func newBSPObs(
	cmpnt attribute.KeyValue,
	qLen func() int64,
	qMax int64,
) (otelconv.SDKProcessorSpanProcessed, metric.Registration, error) {
	meter := otel.GetMeterProvider().Meter(
		selfObsScopeName,
		metric.WithInstrumentationVersion(sdk.Version()),
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	qCap, err := otelconv.NewSDKProcessorSpanQueueCapacity(meter)

	qSize, e := otelconv.NewSDKProcessorSpanQueueSize(meter)
	err = errors.Join(err, e)

	spansProcessed, e := otelconv.NewSDKProcessorSpanProcessed(meter)
	err = errors.Join(err, e)

	cmpntT := semconv.OTelComponentTypeBatchingSpanProcessor
	attrs := metric.WithAttributes(cmpnt, cmpntT)

	reg, e := meter.RegisterCallback(
		func(_ context.Context, o metric.Observer) error {
			o.ObserveInt64(qSize.Inst(), qLen(), attrs)
			o.ObserveInt64(qCap.Inst(), qMax, attrs)
			return nil
		},
		qSize.Inst(),
		qCap.Inst(),
	)
	err = errors.Join(err, e)

	return spansProcessed, reg, err
}

// OnStart method does nothing.
func (*batchSpanProcessor) OnStart(context.Context, ReadWriteSpan) {}

// OnEnd method enqueues a ReadOnlySpan for later processing.
func (bsp *batchSpanProcessor) OnEnd(s ReadOnlySpan) {
	// Do not enqueue spans after Shutdown.
	if bsp.stopped.Load() {
		return
	}

	// Do not enqueue spans if we are just going to drop them.
	if bsp.e == nil {
		return
	}
	bsp.enqueue(s)
}

// Shutdown flushes the queue and waits until all spans are processed.
// It only executes once. Subsequent call does nothing.
func (bsp *batchSpanProcessor) Shutdown(ctx context.Context) error {
	var err error
	bsp.stopOnce.Do(func() {
		bsp.stopped.Store(true)
		wait := make(chan struct{})
		go func() {
			close(bsp.stopCh)
			bsp.stopWait.Wait()
			if bsp.e != nil {
				if err := bsp.e.Shutdown(ctx); err != nil {
					otel.Handle(err)
				}
			}
			close(wait)
		}()
		// Wait until the wait group is done or the context is cancelled
		select {
		case <-wait:
		case <-ctx.Done():
			err = ctx.Err()
		}
		if bsp.selfObservabilityEnabled {
			err = errors.Join(err, bsp.callbackRegistration.Unregister())
		}
	})
	return err
}

type forceFlushSpan struct {
	ReadOnlySpan
	flushed chan struct{}
}

func (forceFlushSpan) SpanContext() trace.SpanContext {
	return trace.NewSpanContext(trace.SpanContextConfig{TraceFlags: trace.FlagsSampled})
}

// ForceFlush exports all ended spans that have not yet been exported.
func (bsp *batchSpanProcessor) ForceFlush(ctx context.Context) error {
	// Interrupt if context is already canceled.
	if err := ctx.Err(); err != nil {
		return err
	}

	// Do nothing after Shutdown.
	if bsp.stopped.Load() {
		return nil
	}

	var err error
	if bsp.e != nil {
		flushCh := make(chan struct{})
		if bsp.enqueueBlockOnQueueFull(ctx, forceFlushSpan{flushed: flushCh}) {
			select {
			case <-bsp.stopCh:
				// The batchSpanProcessor is Shutdown.
				return nil
			case <-flushCh:
				// Processed any items in queue prior to ForceFlush being called
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		wait := make(chan error, 1)
		go func() {
			wait <- bsp.exportSpans(ctx)
		}()
		// Wait until the export is finished or the context is cancelled/timed out
		select {
		case err = <-wait:
		case <-ctx.Done():
			err = ctx.Err()
		}
	}
	return err
}

// WithMaxQueueSize returns a BatchSpanProcessorOption that configures the
// maximum queue size allowed for a BatchSpanProcessor.
func WithMaxQueueSize(size int) BatchSpanProcessorOption {
	return func(o *BatchSpanProcessorOptions) {
		o.MaxQueueSize = size
	}
}

// WithMaxExportBatchSize returns a BatchSpanProcessorOption that configures
// the maximum export batch size allowed for a BatchSpanProcessor.
func WithMaxExportBatchSize(size int) BatchSpanProcessorOption {
	return func(o *BatchSpanProcessorOptions) {
		o.MaxExportBatchSize = size
	}
}

// WithBatchTimeout returns a BatchSpanProcessorOption that configures the
// maximum delay allowed for a BatchSpanProcessor before it will export any
// held span (whether the queue is full or not).
func WithBatchTimeout(delay time.Duration) BatchSpanProcessorOption {
	return func(o *BatchSpanProcessorOptions) {
		o.BatchTimeout = delay
	}
}

// WithExportTimeout returns a BatchSpanProcessorOption that configures the
// amount of time a BatchSpanProcessor waits for an exporter to export before
// abandoning the export.
func WithExportTimeout(timeout time.Duration) BatchSpanProcessorOption {
	return func(o *BatchSpanProcessorOptions) {
		o.ExportTimeout = timeout
	}
}

// WithBlocking returns a BatchSpanProcessorOption that configures a
// BatchSpanProcessor to wait for enqueue operations to succeed instead of
// dropping data when the queue is full.
func WithBlocking() BatchSpanProcessorOption {
	return func(o *BatchSpanProcessorOptions) {
		o.BlockOnQueueFull = true
	}
}

// exportSpans is a subroutine of processing and draining the queue.
func (bsp *batchSpanProcessor) exportSpans(ctx context.Context) error {
	bsp.timer.Reset(bsp.o.BatchTimeout)

	bsp.batchMutex.Lock()
	defer bsp.batchMutex.Unlock()

	if bsp.o.ExportTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(ctx, bsp.o.ExportTimeout, errors.New("processor export timeout"))
		defer cancel()
	}

	if l := len(bsp.batch); l > 0 {
		global.Debug("exporting spans", "count", len(bsp.batch), "total_dropped", atomic.LoadUint32(&bsp.dropped))
		if bsp.selfObservabilityEnabled {
			bsp.spansProcessedCounter.Add(ctx, int64(l),
				bsp.componentNameAttr,
				bsp.spansProcessedCounter.AttrComponentType(otelconv.ComponentTypeBatchingSpanProcessor))
		}
		err := bsp.e.ExportSpans(ctx, bsp.batch)

		// A new batch is always created after exporting, even if the batch failed to be exported.
		//
		// It is up to the exporter to implement any type of retry logic if a batch is failing
		// to be exported, since it is specific to the protocol and backend being sent to.
		clear(bsp.batch) // Erase elements to let GC collect objects
		bsp.batch = bsp.batch[:0]

		if err != nil {
			return err
		}
	}
	return nil
}

// processQueue removes spans from the `queue` channel until processor
// is shut down. It calls the exporter in batches of up to MaxExportBatchSize
// waiting up to BatchTimeout to form a batch.
func (bsp *batchSpanProcessor) processQueue() {
	defer bsp.timer.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		select {
		case <-bsp.stopCh:
			return
		case <-bsp.timer.C:
			if err := bsp.exportSpans(ctx); err != nil {
				otel.Handle(err)
			}
		case sd := <-bsp.queue:
			if ffs, ok := sd.(forceFlushSpan); ok {
				close(ffs.flushed)
				continue
			}
			bsp.batchMutex.Lock()
			bsp.batch = append(bsp.batch, sd)
			shouldExport := len(bsp.batch) >= bsp.o.MaxExportBatchSize
			bsp.batchMutex.Unlock()
			if shouldExport {
				if !bsp.timer.Stop() {
					// Handle both GODEBUG=asynctimerchan=[0|1] properly.
					select {
					case <-bsp.timer.C:
					default:
					}
				}
				if err := bsp.exportSpans(ctx); err != nil {
					otel.Handle(err)
				}
			}
		}
	}
}

// drainQueue awaits the any caller that had added to bsp.stopWait
// to finish the enqueue, then exports the final batch.
func (bsp *batchSpanProcessor) drainQueue() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		select {
		case sd := <-bsp.queue:
			if _, ok := sd.(forceFlushSpan); ok {
				// Ignore flush requests as they are not valid spans.
				continue
			}

			bsp.batchMutex.Lock()
			bsp.batch = append(bsp.batch, sd)
			shouldExport := len(bsp.batch) == bsp.o.MaxExportBatchSize
			bsp.batchMutex.Unlock()

			if shouldExport {
				if err := bsp.exportSpans(ctx); err != nil {
					otel.Handle(err)
				}
			}
		default:
			// There are no more enqueued spans. Make final export.
			if err := bsp.exportSpans(ctx); err != nil {
				otel.Handle(err)
			}
			return
		}
	}
}

func (bsp *batchSpanProcessor) enqueue(sd ReadOnlySpan) {
	ctx := context.TODO()
	if bsp.o.BlockOnQueueFull {
		bsp.enqueueBlockOnQueueFull(ctx, sd)
	} else {
		bsp.enqueueDrop(ctx, sd)
	}
}

func (bsp *batchSpanProcessor) enqueueBlockOnQueueFull(ctx context.Context, sd ReadOnlySpan) bool {
	if !sd.SpanContext().IsSampled() {
		return false
	}

	select {
	case bsp.queue <- sd:
		return true
	case <-ctx.Done():
		if bsp.selfObservabilityEnabled {
			bsp.spansProcessedCounter.Add(ctx, 1,
				bsp.componentNameAttr,
				bsp.spansProcessedCounter.AttrComponentType(otelconv.ComponentTypeBatchingSpanProcessor),
				bsp.spansProcessedCounter.AttrErrorType(queueFull))
		}
		return false
	}
}

func (bsp *batchSpanProcessor) enqueueDrop(ctx context.Context, sd ReadOnlySpan) bool {
	if !sd.SpanContext().IsSampled() {
		return false
	}

	select {
	case bsp.queue <- sd:
		return true
	default:
		atomic.AddUint32(&bsp.dropped, 1)
		if bsp.selfObservabilityEnabled {
			bsp.spansProcessedCounter.Add(ctx, 1,
				bsp.componentNameAttr,
				bsp.spansProcessedCounter.AttrComponentType(otelconv.ComponentTypeBatchingSpanProcessor),
				bsp.spansProcessedCounter.AttrErrorType(queueFull))
		}
	}
	return false
}

// MarshalLog is the marshaling function used by the logging system to represent this Span Processor.
func (bsp *batchSpanProcessor) MarshalLog() any {
	return struct {
		Type         string
		SpanExporter SpanExporter
		Config       BatchSpanProcessorOptions
	}{
		Type:         "BatchSpanProcessor",
		SpanExporter: bsp.e,
		Config:       bsp.o,
	}
}
