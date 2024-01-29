package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

// QueuingStrategy represents a queuing strategy as per the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#qs
type QueuingStrategy struct {
	// HighWaterMark is the maximum number of chunks that can be contained in the internal queue
	// before backpressure is applied.
	HighWaterMark float64 `json:"highWaterMark"`

	// SizeFunc (optional) is a function taking a `chunk` parameter and returning the size
	// of the chunk, in bytes.
	//
	// The result is used to determine backpressure. This function has to be idempotent and
	// not cause side effects.
	Size func(chunk goja.Value) float64 `json:"size"`

	// highWaterMarkSet is true if the highWaterMark was set by the user.
	highWaterMarkSet bool

	// sizeSet is true if the size function was set by the user.
	sizeSet bool
}

// QueuingStrategyBase represents the base of a queuing strategy.
//
// It is meant to be embedded in other queuing strategies.
type QueuingStrategyBase struct{}

// NewQueuingStrategyFrom creates a new queuing strategy from the given goja object.
func NewQueuingStrategyFrom(rt *goja.Runtime, obj *goja.Object) (QueuingStrategy, error) {
	strategy := QueuingStrategy{}

	if common.IsNullish(obj) {
		// If the user didn't provide a queuing strategy, use the default one.
		return strategy, nil
	}

	if err := rt.ExportTo(obj, strategy); err != nil {
		return QueuingStrategy{}, newError(TypeError, "invalid queuing strategy object")
	}

	if !common.IsNullish(obj.Get("highWaterMark")) {
		strategy.highWaterMarkSet = true
	}

	if !common.IsNullish(obj.Get("size")) {
		strategy.sizeSet = true
	}

	return strategy, nil
}

// Implements the [ExtractHighWaterMark] algorithm.
//
// [ExtractHighWaterMark]: https://streams.spec.whatwg.org/#validate-and-normalize-high-water-mark
func (qs *QueuingStrategy) extractHighWaterMark(defaultHWM float64) float64 {
	if qs.highWaterMarkSet {
		return qs.HighWaterMark
	}

	return defaultHWM
}

// Implements the [ExtractSizeAlgorithm] algorithm.
//
// [ExtractSizeAlgorithm]: https://streams.spec.whatwg.org/#make-size-algorithm-from-size-function
func (qs *QueuingStrategy) extractSizeAlgorithm() SizeAlgorithm {
	if qs.sizeSet {
		return func(chunk goja.Value) (float64, error) { return 1.0, nil }
	}

	return func(chunk goja.Value) (float64, error) {
		return qs.Size(chunk), nil
	}
}

// NewCountQueuingStrategy is a common queuing strategy when dealing with streams of
// generic objects is to simply count the number of chunks that have been accumulated
// so far, waiting until this number reaches a specified high-water mark.
//
// See https://streams.spec.whatwg.org/#count-queuing-strategy.
func NewCountQueuingStrategy(highWaterMark float64) QueuingStrategy {
	return QueuingStrategy{
		HighWaterMark: highWaterMark,

		// The default size function returns 1 for any chunk.
		Size: func(chunk goja.Value) float64 { return 1.0 },
	}
}
