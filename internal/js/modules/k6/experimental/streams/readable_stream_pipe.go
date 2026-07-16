package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
)

type streamPipeOptions struct {
	preventAbort  bool
	preventCancel bool
	preventClose  bool
}

func convertStreamPipeOptions(rt *sobek.Runtime, value sobek.Value) streamPipeOptions {
	if value == nil || common.IsNullish(value) {
		return streamPipeOptions{}
	}
	if !isObject(value) {
		throw(rt, newTypeError(rt, "pipe options must be an object"))
	}

	options := value.ToObject(rt)
	// Web IDL dictionary members are observed in lexicographic order.
	preventAbort := pipeOptionBoolean(options.Get("preventAbort"))
	preventCancel := pipeOptionBoolean(options.Get("preventCancel"))
	preventClose := pipeOptionBoolean(options.Get("preventClose"))
	signal := options.Get("signal")
	if signal != nil && !sobek.IsUndefined(signal) {
		throw(rt, newError(NotSupportedError, "AbortSignal is not supported by stream piping yet"))
	}

	return streamPipeOptions{
		preventAbort:  preventAbort,
		preventCancel: preventCancel,
		preventClose:  preventClose,
	}
}

func pipeOptionBoolean(value sobek.Value) bool {
	return value != nil && value.ToBoolean()
}

func (stream *ReadableStream) pipeTo(destination *WritableStream, options streamPipeOptions) *sobek.Promise {
	if stream.isLocked() {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "source stream is locked").Err())
	}
	if destination.isLocked() {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "destination stream is locked").Err())
	}

	pipe := &readableStreamPipeState{
		source:      stream,
		destination: destination,
		reader:      stream.acquireDefaultReader(),
		writer:      destination.acquireDefaultWriter(),
		options:     options,
		result:      newPromiseWrapper(stream.runtime),
	}
	stream.disturbed = true
	pipe.start()
	return pipe.result.promise
}

type readableStreamPipeState struct {
	source      *ReadableStream
	destination *WritableStream
	reader      *ReadableStreamDefaultReader
	writer      *WritableStreamDefaultWriter
	options     streamPipeOptions
	result      *promiseWrapper

	shuttingDown bool
	finalized    bool
	reading      bool
	pumping      bool
	pumpAgain    bool
	waitingReady bool
	sourceDone   bool

	pendingChunkJobs  int
	outstandingWrites int

	action        func() *sobek.Promise
	actionPending bool
	waitForRead   bool
	shutdownErr   any
	hasError      bool
}

func (pipe *readableStreamPipeState) start() {
	rt := pipe.source.runtime

	if _, err := promiseThen(rt, pipe.reader.getClosed().promise,
		func(sobek.Value) { pipe.sourceClosed() },
		func(reason sobek.Value) { pipe.sourceErrored(reason) },
	); err != nil {
		common.Throw(rt, err)
	}
	if _, err := promiseThen(rt, pipe.writer.closedPromise.promise,
		func(sobek.Value) { pipe.destinationClosed() },
		func(reason sobek.Value) { pipe.destinationErrored(reason) },
	); err != nil {
		common.Throw(rt, err)
	}

	// Promise reactions may run synchronously in Sobek, so inspect current state after installing
	// both monitors and rely on the shutdown guard to make duplicate observations harmless.
	switch pipe.source.state {
	case ReadableStreamStateErrored:
		pipe.sourceErrored(pipe.source.storedError)
	case ReadableStreamStateClosed:
		pipe.sourceClosed()
	}
	switch pipe.destination.state {
	case WritableStreamStateErrored, WritableStreamStateErroring:
		pipe.destinationErrored(pipe.destination.storedError)
	case WritableStreamStateClosed:
		pipe.destinationClosed()
	default:
		if pipe.destination.closeQueuedOrInFlight() {
			pipe.destinationClosed()
		}
	}

	// The first pull must happen in a promise job. In particular, pipeTo() must not
	// synchronously invoke a destination write algorithm for an already-queued chunk.
	if _, err := promiseThen(rt, newResolvedPromise(pipe.source.vu, sobek.Undefined()),
		func(sobek.Value) { pipe.pump() }, nil,
	); err != nil {
		common.Throw(rt, err)
	}
}

func (pipe *readableStreamPipeState) pump() {
	if pipe.pumping {
		pipe.pumpAgain = true
		return
	}
	pipe.pumping = true
	defer func() { pipe.pumping = false }()

	for {
		pipe.pumpAgain = false
		if pipe.shuttingDown {
			pipe.finishShutdownWhenReady()
		} else if !pipe.reading {
			desiredSize := pipe.writer.getDesiredSize()
			if common.IsNullish(desiredSize) {
				pipe.destinationErrored(pipe.destination.storedError)
			} else if desiredSize.ToFloat() <= 0 {
				pipe.waitForWriterReady()
			} else {
				pipe.readNextChunk()
			}
		}
		if !pipe.pumpAgain {
			return
		}
	}
}

func (pipe *readableStreamPipeState) waitForWriterReady() {
	if pipe.waitingReady || pipe.shuttingDown {
		return
	}
	pipe.waitingReady = true
	ready := pipe.writer.readyPromise.promise
	if _, err := promiseThen(pipe.source.runtime, ready,
		func(sobek.Value) {
			pipe.waitingReady = false
			pipe.pump()
		},
		func(reason sobek.Value) {
			pipe.waitingReady = false
			pipe.destinationErrored(reason)
		},
	); err != nil {
		common.Throw(pipe.source.runtime, err)
	}
}

func (pipe *readableStreamPipeState) readNextChunk() {
	pipe.reading = true
	pipe.reader.read(ReadRequest{
		chunkSteps: func(chunk any) {
			pipe.reading = false
			pipe.pendingChunkJobs++
			if _, err := promiseThen(
				pipe.source.runtime,
				newResolvedPromise(pipe.source.vu, sobek.Undefined()),
				func(sobek.Value) { pipe.processReadChunk(pipe.source.runtime.ToValue(chunk)) },
				nil,
			); err != nil {
				common.Throw(pipe.source.runtime, err)
			}
		},
		closeSteps: func() {
			pipe.reading = false
			pipe.sourceClosed()
		},
		errorSteps: func(reason any) {
			pipe.reading = false
			pipe.sourceErrored(reason)
		},
	})
}

func (pipe *readableStreamPipeState) processReadChunk(chunk sobek.Value) {
	pipe.pendingChunkJobs--
	if !pipe.shuttingDown ||
		(pipe.destination.state == WritableStreamStateWritable &&
			!pipe.destination.closeQueuedOrInFlight()) {
		pipe.writeChunk(chunk)
	}
	if pipe.shuttingDown {
		pipe.finishShutdownWhenReady()
		return
	}
	pipe.pump()
}

func (pipe *readableStreamPipeState) writeChunk(chunk sobek.Value) {
	// Increment before invoking the writer. The sink's write algorithm can resolve another
	// promise re-entrantly in Sobek, and that reaction can error the source before write()
	// returns its own promise.
	pipe.outstandingWrites++
	writePromise := pipe.writer.write(chunk)
	if _, err := promiseThen(pipe.source.runtime, writePromise,
		func(sobek.Value) {
			pipe.outstandingWrites--
			if pipe.shuttingDown {
				pipe.finishShutdownWhenReady()
			}
		},
		func(reason sobek.Value) {
			pipe.outstandingWrites--
			pipe.destinationErrored(reason)
		},
	); err != nil {
		common.Throw(pipe.source.runtime, err)
	}
}

func (pipe *readableStreamPipeState) sourceClosed() {
	if pipe.shuttingDown {
		pipe.finishShutdownWhenReady()
		return
	}
	pipe.sourceDone = true
	if pipe.options.preventClose {
		pipe.shutdown(nil, false, nil, true)
		return
	}
	pipe.shutdown(pipe.writer.closeWithErrorPropagation, false, nil, true)
}

func (pipe *readableStreamPipeState) sourceErrored(reason any) {
	if pipe.shuttingDown {
		pipe.finishShutdownWhenReady()
		return
	}
	if pipe.options.preventAbort {
		pipe.shutdown(nil, true, reason, true)
		return
	}
	pipe.shutdown(func() *sobek.Promise {
		return pipe.writer.abort(pipe.source.runtime.ToValue(throwableValue(reason)))
	}, true, reason, true)
}

func (pipe *readableStreamPipeState) destinationErrored(reason any) {
	if pipe.shuttingDown {
		pipe.finishShutdownWhenReady()
		return
	}
	if pipe.options.preventCancel {
		pipe.shutdown(nil, true, reason, false)
		return
	}
	pipe.shutdown(func() *sobek.Promise {
		return pipe.reader.cancel(pipe.source.runtime.ToValue(throwableValue(reason)))
	}, true, reason, false)
}

func (pipe *readableStreamPipeState) destinationClosed() {
	if pipe.shuttingDown {
		pipe.finishShutdownWhenReady()
		return
	}
	if pipe.sourceDone {
		return
	}
	reason := newTypeError(pipe.source.runtime, "destination stream closed before the source")
	if pipe.options.preventCancel {
		pipe.shutdown(nil, true, reason, false)
		return
	}
	pipe.shutdown(func() *sobek.Promise {
		return pipe.reader.cancel(pipe.source.runtime.ToValue(reason.Err()))
	}, true, reason, false)
}

func (pipe *readableStreamPipeState) shutdown(
	action func() *sobek.Promise,
	hasError bool,
	reason any,
	waitForRead bool,
) {
	if pipe.shuttingDown {
		return
	}
	pipe.shuttingDown = true
	pipe.action = action
	pipe.hasError = hasError
	pipe.shutdownErr = reason
	pipe.waitForRead = waitForRead
	pipe.finishShutdownWhenReady()
}

func (pipe *readableStreamPipeState) finishShutdownWhenReady() {
	if pipe.finalized || pipe.actionPending || (pipe.waitForRead && pipe.reading) ||
		pipe.pendingChunkJobs != 0 ||
		pipe.outstandingWrites != 0 {
		return
	}
	if pipe.action == nil {
		pipe.finalize(pipe.hasError, pipe.shutdownErr)
		return
	}

	action := pipe.action
	pipe.action = nil
	pipe.actionPending = true
	actionPromise := action()
	if _, err := promiseThen(pipe.source.runtime, actionPromise,
		func(sobek.Value) {
			pipe.actionPending = false
			pipe.finalize(pipe.hasError, pipe.shutdownErr)
		},
		func(reason sobek.Value) {
			pipe.actionPending = false
			pipe.finalize(true, reason)
		},
	); err != nil {
		common.Throw(pipe.source.runtime, err)
	}
}

func (pipe *readableStreamPipeState) finalize(hasError bool, reason any) {
	if pipe.finalized {
		return
	}
	pipe.finalized = true
	pipe.writer.release()
	pipe.reader.release()
	if hasError {
		pipe.result.rejectWith(throwableValue(reason))
	} else {
		pipe.result.resolveWith(sobek.Undefined())
	}
}
