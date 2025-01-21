package streams

import (
	"errors"
	"math"

	"github.com/grafana/sobek"
)

// ValueWithSize holds a value and its corresponding size.
//
// It is used to store values in the queue.
type ValueWithSize struct {
	Value sobek.Value
	Size  float64
}

// QueueWithSizes is a queue of values with sizes.
type QueueWithSizes struct {
	Queue          []ValueWithSize
	QueueTotalSize float64
	runtime        *sobek.Runtime
}

// NewQueueWithSizes creates a new queue of values with sizes, as described in the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#queue-with-sizes
func NewQueueWithSizes(runtime *sobek.Runtime) *QueueWithSizes {
	return &QueueWithSizes{
		Queue:   make([]ValueWithSize, 0),
		runtime: runtime,
	}
}

// Enqueue adds a value to the queue, and implements the specification's [EnqueueValueWithSize] abstract operation.
//
// [EnqueueValueWithSize]: https://streams.spec.whatwg.org/#enqueue-value-with-size
func (q *QueueWithSizes) Enqueue(value sobek.Value, size float64) error {
	if math.IsNaN(size) || size < 0 || math.IsInf(size, 1) { // Check for +Inf
		return newRangeError(q.runtime, "size must be a finite, non-NaN number")
	}

	valueWithSize := ValueWithSize{
		Value: value,
		Size:  size,
	}

	q.Queue = append(q.Queue, valueWithSize)
	q.QueueTotalSize += size

	return nil
}

// Dequeue removes and returns the first value from the queue.
//
// It implements the [DequeueValue] abstract operation.
//
// [DequeueValue]: https://streams.spec.whatwg.org/#abstract-opdef-dequeue-value
func (q *QueueWithSizes) Dequeue() (sobek.Value, error) {
	if len(q.Queue) == 0 {
		return nil, newError(AssertionError, "queue is empty")
	}

	valueWithSize := q.Queue[0]
	q.Queue = q.Queue[1:]
	q.QueueTotalSize -= valueWithSize.Size
	if q.QueueTotalSize < 0 {
		q.QueueTotalSize = 0 // Correct for rounding errors
	}

	return valueWithSize.Value, nil
}

// Peek returns the first value from the queue without removing it.
//
// It implements the [PeekQueueValue] abstract operation.
//
// [PeekQueueValue]: https://streams.spec.whatwg.org/#abstract-opdef-peek-queue-value
func (q *QueueWithSizes) Peek() (sobek.Value, error) {
	if len(q.Queue) == 0 {
		return nil, errors.New("queue is empty")
	}

	return q.Queue[0].Value, nil
}

// Reset clears the queue and resets the total size.
//
// It implements the [ResetQueue] abstract operation.
//
// [ResetQueue]: https://streams.spec.whatwg.org/#abstract-opdef-reset-queue
func (q *QueueWithSizes) Reset() {
	q.Queue = make([]ValueWithSize, 0)
	q.QueueTotalSize = 0
}

// Len returns the length of the queue.
func (q *QueueWithSizes) Len() int {
	return len(q.Queue)
}
