package lib

import (
	"encoding"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// ExecutionSegment represents a (start, end] partition of the total execution
// work for a specific test. For example, if we want the split the execution of a
// test in 2 different parts, we can split it in two segments (0, 0.5] and (0,5, 1].
//
// We use rational numbers so it's easier to verify the correctness and easier to
// reason about portions of indivisible things, like VUs. This way, we can easily
// split a test in thirds (i.e. (0, 1/3], (1/3, 2/3], (2/3, 1]), without fearing
// that we'll lose a VU along the way...
//
// The most important part is that if work is split between multiple k6 instances,
// each k6 instance can precisely and reproducibly calculate its share of the work,
// just by knowing its own segment. There won't be a need to schedule the
// execution from a master node, or to even know how many other k6 instances are
// running!
type ExecutionSegment struct {
	// 0 <= from < to <= 1
	from *big.Rat
	to   *big.Rat

	// derived, equals to-from, but pre-calculated here for speed
	length *big.Rat
}

// Ensure we implement those interfaces
var (
	_ encoding.TextUnmarshaler = &ExecutionSegment{}
	_ fmt.Stringer             = &ExecutionSegment{}
)

// Helpful "constants" so we don't initialize them in every function call
var (
	zeroRat, oneRat      = big.NewRat(0, 1), big.NewRat(1, 1) //nolint:gochecknoglobals
	oneBigInt, twoBigInt = big.NewInt(1), big.NewInt(2)       //nolint:gochecknoglobals
)

// NewExecutionSegment validates the supplied arguments (basically, that 0 <=
// from < to <= 1) and either returns an error, or it returns a
// fully-initialized and usable execution segment.
func NewExecutionSegment(from, to *big.Rat) (*ExecutionSegment, error) {
	if from.Cmp(zeroRat) < 0 {
		return nil, fmt.Errorf("segment start value must be at least 0 but was %s", from.FloatString(2))
	}
	if from.Cmp(to) >= 0 {
		return nil, fmt.Errorf("segment start(%s) must be less than its end(%s)", from.FloatString(2), to.FloatString(2))
	}
	if to.Cmp(oneRat) > 0 {
		return nil, fmt.Errorf("segment end value can't be more than 1 but was %s", to.FloatString(2))
	}
	return newExecutionSegment(from, to), nil
}

// newExecutionSegment just creates an ExecutionSegment without validating the arguments
func newExecutionSegment(from, to *big.Rat) *ExecutionSegment {
	return &ExecutionSegment{
		from:   from,
		to:     to,
		length: new(big.Rat).Sub(to, from),
	}
}

// stringToRat is a helper function that tries to convert a string to a rational
// number while allowing percentage, decimal, and fraction values.
func stringToRat(s string) (*big.Rat, error) {
	if strings.HasSuffix(s, "%") {
		num, ok := new(big.Int).SetString(strings.TrimSuffix(s, "%"), 10)
		if !ok {
			return nil, fmt.Errorf("'%s' is not a valid percentage", s)
		}
		return new(big.Rat).SetFrac(num, big.NewInt(100)), nil
	}
	rat, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("'%s' is not a valid percentage, decimal, fraction or interval value", s)
	}
	return rat, nil
}

// NewExecutionSegmentFromString validates the supplied string value and returns
// the newly created ExecutionSegment or and error from it.
//
// We are able to parse both single percentage/float/fraction values, and actual
// (from: to] segments. For the single values, we just treat them as the
// beginning segment - thus the execution segment can be used as a shortcut for
// quickly running an arbitrarily scaled-down version of a test.
//
// The parsing logic is that values with a colon, i.e. ':', are full segments:
//
//	`1/2:3/4`, `0.5:0.75`, `50%:75%`, and even `2/4:75%` should be (1/2, 3/4]
//
// And values without a colon are the end of a first segment:
//
//	`20%`, `0.2`,  and `1/5` should be converted to (0, 1/5]
//
// empty values should probably be treated as "1", i.e. the whole execution
func NewExecutionSegmentFromString(toStr string) (result *ExecutionSegment, err error) {
	from := zeroRat
	if toStr == "" {
		toStr = "1" // an empty string means a full 0:1 execution segment
	}
	if strings.ContainsRune(toStr, ':') {
		fromToStr := strings.SplitN(toStr, ":", 2)
		toStr = fromToStr[1]
		if from, err = stringToRat(fromToStr[0]); err != nil {
			return nil, err
		}
	}

	to, err := stringToRat(toStr)
	if err != nil {
		return nil, err
	}

	return NewExecutionSegment(from, to)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface, so that
// execution segments can be specified as CLI flags, environment variables, and
// JSON strings. It is a wrapper for the NewExecutionFromString() constructor.
func (es *ExecutionSegment) UnmarshalText(text []byte) (err error) {
	segment, err := NewExecutionSegmentFromString(string(text))
	if err != nil {
		return err
	}
	*es = *segment
	return nil
}

func (es *ExecutionSegment) String() string {
	if es == nil {
		return "0:1"
	}
	return es.from.RatString() + ":" + es.to.RatString()
}

// MarshalText implements the encoding.TextMarshaler interface, so is used for
// text and JSON encoding of the execution segment.
func (es *ExecutionSegment) MarshalText() ([]byte, error) {
	if es == nil {
		return nil, nil
	}
	return []byte(es.String()), nil
}

// FloatLength is a helper method for getting some more human-readable
// information about the execution segment.
func (es *ExecutionSegment) FloatLength() float64 {
	if es == nil {
		return 1.0
	}
	res, _ := es.length.Float64()
	return res
}

// Split evenly divides the execution segment into the specified number of
// equal consecutive execution sub-segments.
func (es *ExecutionSegment) Split(numParts int64) ([]*ExecutionSegment, error) {
	if numParts < 1 {
		return nil, fmt.Errorf("the number of parts must be at least 1, %d received", numParts)
	}

	from, to := zeroRat, oneRat
	if es != nil {
		from, to = es.from, es.to
	}

	increment := new(big.Rat).Sub(to, from)
	increment.Denom().Mul(increment.Denom(), big.NewInt(numParts))

	results := make([]*ExecutionSegment, numParts)
	for i := int64(0); i < numParts; i++ {
		segmentTo := new(big.Rat).Add(from, increment)
		segment, err := NewExecutionSegment(from, segmentTo)
		if err != nil {
			return nil, err
		}
		results[i] = segment
		from = segmentTo
	}

	if from.Cmp(to) != 0 {
		return nil, fmt.Errorf("expected %s and %s to be equal", from, to)
	}

	return results, nil
}

// Equal returns true only if the two execution segments have the same from and
// to values.
func (es *ExecutionSegment) Equal(other *ExecutionSegment) bool {
	if es == other {
		return true
	}
	thisFrom, otherFrom, thisTo, otherTo := zeroRat, zeroRat, oneRat, oneRat
	if es != nil {
		thisFrom, thisTo = es.from, es.to
	}
	if other != nil {
		otherFrom, otherTo = other.from, other.to
	}
	return thisFrom.Cmp(otherFrom) == 0 && thisTo.Cmp(otherTo) == 0
}

// SubSegment returns a new execution sub-segment - if a is (1/2:1] and b is
// (0:1/2], then a.SubSegment(b) will return a new segment (1/2, 3/4].
//
// The basic formula for c = a.SubSegment(b) is:
//
//	c.from = a.from + b.from * (a.to - a.from)
//	c.to = c.from + (b.to - b.from) * (a.to - a.from)
func (es *ExecutionSegment) SubSegment(child *ExecutionSegment) *ExecutionSegment {
	if child == nil {
		return es // 100% sub-segment is the original segment
	}

	parentFrom, parentLength := zeroRat, oneRat
	if es != nil {
		parentFrom, parentLength = es.from, es.length
	}

	resultFrom := new(big.Rat).Mul(parentLength, child.from)
	resultFrom.Add(resultFrom, parentFrom)

	resultLength := new(big.Rat).Mul(parentLength, child.length)
	return &ExecutionSegment{
		from:   resultFrom,
		length: resultLength,
		to:     new(big.Rat).Add(resultFrom, resultLength),
	}
}

// helper function for rounding (up) of rational numbers to big.Int values
func roundUp(rat *big.Rat) *big.Int {
	quo, rem := new(big.Int).QuoRem(rat.Num(), rat.Denom(), new(big.Int))

	if rem.Mul(rem, twoBigInt).Cmp(rat.Denom()) >= 0 {
		return quo.Add(quo, oneBigInt)
	}
	return quo
}

// Scale proportionally scales the supplied value, according to the execution
// segment's position and size of the work.
func (es *ExecutionSegment) Scale(value int64) int64 {
	if es == nil { // no execution segment, i.e. 100%
		return value
	}
	// Instead of the first proposal that used remainders and floor:
	//    floor( (value * from) % 1 + value * length )
	// We're using an alternative approach with rounding that (hopefully) has
	// the same properties, but it's simpler and has better precision:
	//    round( (value * from) - round(value * from) + (value * (to - from)) )?
	// which reduces to:
	//    round( (value * to) - round(value * from) )?

	toValue := big.NewRat(value, 1)
	toValue.Mul(toValue, es.to)

	fromValue := big.NewRat(value, 1)
	fromValue.Mul(fromValue, es.from)

	toValue.Sub(toValue, new(big.Rat).SetFrac(roundUp(fromValue), oneBigInt))

	return roundUp(toValue).Int64()
}

// InPlaceScaleRat scales rational numbers in-place - it changes the passed
// argument (and also returns it, to allow for chaining, like many other big.Rat
// methods).
func (es *ExecutionSegment) InPlaceScaleRat(value *big.Rat) *big.Rat {
	if es == nil { // no execution segment, i.e. 100%
		return value
	}
	return value.Mul(value, es.length)
}

// CopyScaleRat scales rational numbers without changing them - creates a new
// bit.Rat object and uses it for the calculation.
func (es *ExecutionSegment) CopyScaleRat(value *big.Rat) *big.Rat {
	if es == nil { // no execution segment, i.e. 100%
		return value
	}
	return new(big.Rat).Mul(value, es.length)
}

// ExecutionSegmentSequence represents an ordered chain of execution segments,
// where the end of one segment is the beginning of the next. It can serialized
// as a comma-separated string of rational numbers "r1,r2,r3,...,rn", which
// represents the sequence (r1, r2], (r2, r3], (r3, r4], ..., (r{n-1}, rn].
// The empty value should be treated as if there is a single (0, 1] segment.
type ExecutionSegmentSequence []*ExecutionSegment

// NewExecutionSegmentSequence validates the that the supplied execution
// segments are non-overlapping and without gaps. It will return a new execution
// segment sequence if that is true, and an error if it's not.
func NewExecutionSegmentSequence(segments ...*ExecutionSegment) (ExecutionSegmentSequence, error) {
	if len(segments) > 1 {
		to := segments[0].to
		for i, segment := range segments[1:] {
			if segment.from.Cmp(to) != 0 {
				return nil, fmt.Errorf(
					"the start value %s of segment #%d must be equal to the end value of the previous one, but it is %s",
					segment.from, i+1, to,
				)
			}
			to = segment.to
		}
	}
	return ExecutionSegmentSequence(segments), nil
}

// NewExecutionSegmentSequenceFromString parses strings of the format
// "r1,r2,r3,...,rn", which represents the sequences like (r1, r2], (r2, r3],
// (r3, r4], ..., (r{n-1}, rn].
func NewExecutionSegmentSequenceFromString(strSeq string) (ExecutionSegmentSequence, error) {
	if len(strSeq) == 0 {
		return nil, nil
	}

	points := strings.Split(strSeq, ",")
	if len(points) < 2 {
		return nil, fmt.Errorf("at least 2 points are needed for an execution segment sequence, %d given", len(points))
	}
	var start *big.Rat

	segments := make([]*ExecutionSegment, 0, len(points)-1)
	for i, point := range points {
		rat, err := stringToRat(point)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			start = rat
			continue
		}

		segment, err := NewExecutionSegment(start, rat)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
		start = rat
	}

	return NewExecutionSegmentSequence(segments...)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface, so that
// execution segment sequences can be specified as CLI flags, environment
// variables, and JSON strings.
func (ess *ExecutionSegmentSequence) UnmarshalText(text []byte) (err error) {
	seq, err := NewExecutionSegmentSequenceFromString(string(text))
	if err != nil {
		return err
	}
	*ess = seq
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface, so is used for
// text and JSON encoding of the execution segment sequences.
func (ess ExecutionSegmentSequence) MarshalText() ([]byte, error) {
	return []byte(ess.String()), nil
}

// String just implements the fmt.Stringer interface, encoding the sequence of
// segments as "start1,end1,end2,end3,...,endn".
func (ess ExecutionSegmentSequence) String() string {
	result := make([]string, 0, len(ess)+1)
	for i, s := range ess {
		if i == 0 {
			result = append(result, s.from.RatString())
		}
		result = append(result, s.to.RatString())
	}
	return strings.Join(result, ",")
}

// LCD calculates the lowest common denominator of the sequence.
// https://en.wikipedia.org/wiki/Least_common_multiple#Using_the_greatest_common_divisor
func (ess ExecutionSegmentSequence) LCD() int64 {
	acc := ess[0].length.Denom().Int64()
	var n int64
	for _, seg := range ess[1:] {
		n = seg.length.Denom().Int64()
		if acc == n || acc%n == 0 { // short circuit
			continue
		}
		acc *= (n / gcd(acc, n))
	}

	return acc
}

// Greatest common divisor
// https://en.wikipedia.org/wiki/Euclidean_algorithm
func gcd(a, b int64) int64 {
	for a != b {
		if a > b {
			a -= b
		} else {
			b -= a
		}
	}
	return a
}

// IsFull returns whether the sequences is full, that is, whether it starts at 0
// and ends at 1. Use GetFilledExecutionSegmentSequence() to get a full sequence.
func (ess ExecutionSegmentSequence) IsFull() bool {
	return ess != nil && len(ess) != 0 && ess[0].from.Cmp(zeroRat) == 0 && ess[len(ess)-1].to.Cmp(oneRat) == 0
}

// FindSegmentPosition returns the index of the supplied execution segment in
// the sequence, or an error if the segment isn't present. This shouldn't be
// used on a nil or empty sequence, it's best to use this method on the result
// of GetFilledExecutionSegmentSequence().
func (ess ExecutionSegmentSequence) FindSegmentPosition(segment *ExecutionSegment) (int, error) {
	from := zeroRat
	if segment != nil {
		from = segment.from
	}
	index := sort.Search(len(ess), func(i int) bool {
		return ess[i].from.Cmp(from) >= 0
	})

	if index < 0 || index >= len(ess) || !ess[index].Equal(segment) {
		return -1, fmt.Errorf("couldn't find segment %s in sequence %s", segment, ess)
	}
	return index, nil
}

// GetFilledExecutionSegmentSequence makes sure we don't have any gaps in the
// given execution segment sequence, or a nil one. It makes sure that the whole
// 0-1 range is filled.
func GetFilledExecutionSegmentSequence(
	sequence *ExecutionSegmentSequence, fallback *ExecutionSegment,
) (result ExecutionSegmentSequence) {
	if sequence == nil || len(*sequence) == 0 {
		if fallback == nil || fallback.length.Cmp(oneRat) == 0 {
			// There is no sequence or a segment, so it means the whole test run
			// is being planned/executed. So we make sure not to have a nil
			// sequence, returning a full; "0,1" sequence instead, otherwise we
			// will need to check for nil everywhere...
			return ExecutionSegmentSequence{newExecutionSegment(zeroRat, oneRat)}
		}
		// We don't have a sequence, but we have a defined segment, so we
		// fill around it with the missing pieces for a full sequence.
		result = ExecutionSegmentSequence{fallback}
	} else {
		result = *sequence
	}

	if result[0].from.Cmp(zeroRat) != 0 {
		es := newExecutionSegment(zeroRat, result[0].from)
		result = append(ExecutionSegmentSequence{es}, result...)
	}

	if result[len(result)-1].to.Cmp(oneRat) != 0 {
		es := newExecutionSegment(result[len(result)-1].to, oneRat)
		result = append(result, es)
	}
	return result
}

// ExecutionSegmentSequenceWrapper is a caching layer on top of the execution
// segment sequence that allows us to make fast and useful calculations, after
// a somewhat slow initialization.
type ExecutionSegmentSequenceWrapper struct {
	ExecutionSegmentSequence       // a filled-out segment sequence
	lcd                      int64 // pre-calculated least common denominator

	// The striped offsets, i.e. the repeating indexes that "belong" to each
	// execution segment in the sequence.
	offsets [][]int64
}

// NewExecutionSegmentSequenceWrapper expects a filled-out execution segment
// sequence. It pre-calculates the initial caches of and returns a new
// ExecutionSegmentSequenceWrapper, but doesn't calculate the striped offsets.
func NewExecutionSegmentSequenceWrapper(ess ExecutionSegmentSequence) *ExecutionSegmentSequenceWrapper {
	if !ess.IsFull() {
		panic(fmt.Sprintf("Cannot wrap around a non-full execution segment sequence '%s'", ess))
	}

	sequenceLength := len(ess)
	offsets := make([][]int64, sequenceLength)
	lcd := ess.LCD()

	// This will contain the normalized numerator values (i.e. what they would have
	// been if all denominators were equal to the LCD), sorted in descending
	// order (i.e. biggest segments are first), with references to their actual
	// indexes in the execution segment sequence (i.e. `seq` above).
	sortedNormalizedIndexes := make([]struct {
		normNumerator int64
		originalIndex int
	}, sequenceLength)

	for i := range ess {
		normalizedNumerator := ess[i].length.Num().Int64() * (lcd / ess[i].length.Denom().Int64())
		sortedNormalizedIndexes[i].normNumerator = normalizedNumerator
		sortedNormalizedIndexes[i].originalIndex = i
		offsets[i] = make([]int64, 0, normalizedNumerator+1)
	}

	sort.SliceStable(sortedNormalizedIndexes, func(i, j int) bool {
		return sortedNormalizedIndexes[i].normNumerator > sortedNormalizedIndexes[j].normNumerator
	})

	// This is the striping algorithm. Imagine you have a number of rational
	// numbers which all add up to 1 (or less), and call them segments. If you
	// want each to get proportional amount of anything, you need to give them
	// their numerator count of elements for each denominator amount from the
	// original elements. So, for 1/3, you give 1 element for each 3 elements.
	// For 3/5 - 3 elements for each 5. If you have, for example, a sequence
	// with elements with length 3/5 and 1/3, in order to know how to distribute
	// it accurately, you need to get the LCD(lowest common denominitor). In
	// this case, between 3 and 5, the LCD is 15. Then to transform the numbers
	// to have the same, LCD equal, denominator. So 3/5 becomes 9/15 and 1/3
	// becomes 5/15. So now for each 15 elements 9 need to go to the 3/5, and 5
	// need to go to 1/3. This is what we did above in sortedNormalizedIndexes.
	//
	// We use the algorithm below to split elements between ExecutionSegments by
	// using their length as the rational number. As we would like to get
	// non-sequential elements, we try to get the maximum distance between them.
	// That is the number of elements divided by the number of elements for any
	// given segment, which concidently is the length of the segment reversed.
	// The algorithm below does the following:
	//  1. Goes through the elements from 0 to the lcd-1
	//  2. For each of element, it goes through the segments and looks if the
	//     amount of already taken elements by the given segment, multiplied by
	//     that segment's length inverted, is equal to or less to the current
	//     element index. If it is, give that element to that segment. If not,
	//     continue with the next element.
	// The code below specifically avoids using big.Rat, for performance
	// reasons, which complicates the code somewhat. As additional note, the
	// sorting of the segments from biggest to smallest helps with the fact that
	// the biggest elements will need to take the most elements, and for them it
	// will be the hardest to not get sequential elements.
	prev := make([]int64, sequenceLength)
	chosenCounts := make([]int64, sequenceLength)
	saveIndex := func(iteration int64, index int, numerator int64) {
		offsets[index] = append(offsets[index], iteration-prev[index])
		prev[index] = iteration
		if int64(len(offsets[index])) == numerator {
			offsets[index] = append(offsets[index], offsets[index][0]+lcd-iteration)
		}
	}
	for i := int64(0); i < lcd; i++ {
		for sortedIndex, chosenCount := range chosenCounts {
			num := chosenCount * lcd
			denom := sortedNormalizedIndexes[sortedIndex].normNumerator
			if i > num/denom || (i == num/denom && num%denom == 0) {
				chosenCounts[sortedIndex]++
				saveIndex(i, sortedNormalizedIndexes[sortedIndex].originalIndex, denom)
				break
			}
		}
	}

	return &ExecutionSegmentSequenceWrapper{ExecutionSegmentSequence: ess, lcd: lcd, offsets: offsets}
}

// LCD returns the (cached) least common denominator of the sequence - no need
// to calculate it again, since we did it in the constructor.
func (essw *ExecutionSegmentSequenceWrapper) LCD() int64 {
	return essw.lcd
}

// ScaleInt64 scales the provided value for the given segment.
func (essw *ExecutionSegmentSequenceWrapper) ScaleInt64(segmentIndex int, value int64) int64 {
	start := essw.offsets[segmentIndex][0]
	offsets := essw.offsets[segmentIndex][1:]
	result := (value / essw.lcd) * int64(len(offsets))
	for gi, i := 0, start; i < value%essw.lcd; gi, i = gi+1, i+offsets[gi] {
		result++
	}
	return result
}

// GetStripedOffsets returns the stripped offsets for the given segment
// the returned values are as follows in order:
//   - start: the first value that is for the segment
//   - offsets: a list of offsets from the previous value for the segment. This are only the offsets
//     to from the start to the next start if we chunk the elements we are going to strip
//     into lcd sized chunks
//   - lcd: the LCD of the lengths of all segments in the sequence. This is also the number of
//     elements after which the algorithm starts to loop and give the same values
func (essw *ExecutionSegmentSequenceWrapper) GetStripedOffsets(segmentIndex int) (int64, []int64, int64) {
	offsets := essw.offsets[segmentIndex]
	return offsets[0], offsets[1:], essw.lcd
}

// GetTuple returns an ExecutionTuple for the specified segment index.
func (essw *ExecutionSegmentSequenceWrapper) GetTuple(segmentIndex int) *ExecutionTuple {
	return &ExecutionTuple{
		Sequence:     essw,
		Segment:      essw.ExecutionSegmentSequence[segmentIndex],
		SegmentIndex: segmentIndex,
	}
}

// GetNewExecutionSegmentSequenceFromValue uses the value provided, splits it
// between all the segments, using the striping offsets in the sequence,
// generating a new segment sequence. It then returns a new
// ExecutionSegmentSequenceWrapper, with the new sequence and segments, such
// that each new segment in the new sequence has length `Scale(value)/value`
// while keeping the order.
//
// Additionally, the position of a given segment index can be tracked (since
// empty segments are removed), so that you can reconstruct an ExecutionTuple,
// if required. If the segment with the trackedIndex is not part of the new
// sequence, or if a new sequence cannot be generated (for example, for 0
// values), an error will be returned.
func (essw *ExecutionSegmentSequenceWrapper) GetNewExecutionSegmentSequenceFromValue(value int64, trackedIndex int) (
	newSequence *ExecutionSegmentSequenceWrapper, newIndex int, err error,
) {
	if value < 1 {
		return nil, -1, fmt.Errorf("cannot generate new sequence for value %d", value)
	}

	if value%essw.lcd == 0 { // the value is perfectly divisible so we will get the same tuple
		return essw, trackedIndex, nil
	}

	newIndex = -1
	newESS := make(ExecutionSegmentSequence, 0, len(essw.ExecutionSegmentSequence)) // this can be smaller

	prev := int64(0)
	for i := range essw.ExecutionSegmentSequence {
		newValue := essw.ScaleInt64(i, value)
		if newValue == 0 {
			continue
		}
		currentES := newExecutionSegment(big.NewRat(prev, value), big.NewRat(prev+newValue, value))
		prev += newValue
		if i == trackedIndex {
			newIndex = len(newESS)
		}
		newESS = append(newESS, currentES)
	}

	if newIndex == -1 {
		return nil, -1, fmt.Errorf(
			"segment %d (%s) isn't present in the new sequence",
			trackedIndex, essw.ExecutionSegmentSequence[trackedIndex],
		)
	}

	return NewExecutionSegmentSequenceWrapper(newESS), newIndex, nil
}

// ExecutionTuple is the combination of an ExecutionSegmentSequence(Wrapper) and
// a specific ExecutionSegment from it. It gives easy access to the efficient
// scaling and striping algorithms for that specific segment, since the results
// are cached in the sequence wrapper. It is also the basis for the
// SegmentedIndex type below, which is the actual implementation of a segmented
// (striped) iterator, usable both for segmenting actual iterations and for
// partitioning data between multiple instances.
//
// For example, let's try to segment a load test in 3 unequal parts: 50%, 25%
// and 25% (so the ExecutionSegmentSequence will contain these segments: 0:1/2,
// 1/2:3/4, 3/4:1). The building blocks that k6 needs for distributed execution
// are segmented (non-overlapping) iterators and proportionally dividing integer
// numbers as fairly as possible between multiple segments in a stable manner.
//
// The segmented iterators (i.e. SegmentedIndex values below) will be like this:
//
//	Normal iterator:              0   1   2   3   4   5   6   7   8   9   10  11 ...
//	Instance 1 (0:1/2) iterator:  0       2       4       6       8       10     ...
//	Instance 2 (1/2:3/4) iterator:    1               5               9          ...
//	Instance 2 (3/4:1) iterator:              3               7               11 ...
//
// See how every instance has its own uniqe non-overlapping iterator, but when
// we combine all of them, we cover every possible value in the original one.
//
// We also can use this property to scale integer numbers proportionally, as
// fairly as possible, between the instances, like this:
//
//	Global int value to scale:    1   2   3   4   5   6   7   8   9   10  11  12 ...
//	Calling ScaleInt64():
//	- Instance 1 (0:1/2) value:   1   1   2   2   3   3   4   4   5   5   6   6  ...
//	- Instance 2 (1/2:3/4) value: 0   1   1   1   1   2   2   2   2   3   3   3  ...
//	- Instance 2 (3/4:1) value:   0   0   0   1   1   1   1   2   2   2   2   3  ...
//
// Notice how the sum of the per-instance values is always equal to the global
// value - this is what ExecutionTuple.ScaleInt64() does. Also compare both
// tables (their positions match), see how we only increment the value for a
// particular instance when we would have cycled the iterator on that step.
//
// This also makes the scaling stable, in contrast to ExecutionSegment.Scale().
// Scaled values will only ever increase, since we just increment them in a
// specific order between instances. There will never be a situation where
// `ScaleInt64(i)` is less than `ScaleInt64(i+n)` for any positive n!
//
// The algorithm that calculates the offsets and everything that's necessary to
// have these segmented iterators is in NewExecutionSegmentSequenceWrapper().
// The ExecutionTuple simply exposes them efficiently for a single segment of
// the sequence, so it's the thing that most users will probably need.
type ExecutionTuple struct { // TODO rename? make fields private and have getter methods?
	Sequence     *ExecutionSegmentSequenceWrapper
	Segment      *ExecutionSegment
	SegmentIndex int
}

func (et *ExecutionTuple) String() string {
	return fmt.Sprintf("%s in %s", et.Segment, et.Sequence)
}

// NewExecutionTuple returns a new ExecutionTuple for the provided segment and
// sequence.
//
// TODO: don't return a pointer?
func NewExecutionTuple(segment *ExecutionSegment, sequence *ExecutionSegmentSequence) (*ExecutionTuple, error) {
	filledSeq := GetFilledExecutionSegmentSequence(sequence, segment)
	wrapper := NewExecutionSegmentSequenceWrapper(filledSeq)
	index, err := wrapper.FindSegmentPosition(segment)
	if err != nil {
		return nil, err
	}
	return &ExecutionTuple{Sequence: wrapper, Segment: segment, SegmentIndex: index}, nil
}

// ScaleInt64 scales the provided value for our execution segment.
func (et *ExecutionTuple) ScaleInt64(value int64) int64 {
	if len(et.Sequence.ExecutionSegmentSequence) == 1 {
		return value // if we don't have any segmentation, just return the original value
	}
	return et.Sequence.ScaleInt64(et.SegmentIndex, value)
}

// GetStripedOffsets returns the striped offsets for our execution segment.
func (et *ExecutionTuple) GetStripedOffsets() (int64, []int64, int64) {
	return et.Sequence.GetStripedOffsets(et.SegmentIndex)
}

// GetNewExecutionTupleFromValue re-segments the sequence, based on the given
// value (see GetNewExecutionSegmentSequenceFromValue() above), and either
// returns the new tuple, or an error if the current segment isn't present in
// the new sequence.
func (et *ExecutionTuple) GetNewExecutionTupleFromValue(value int64) (*ExecutionTuple, error) {
	newSequenceWrapper, newIndex, err := et.Sequence.GetNewExecutionSegmentSequenceFromValue(value, et.SegmentIndex)
	if err != nil {
		return nil, err
	}
	return &ExecutionTuple{
		Sequence:     newSequenceWrapper,
		Segment:      newSequenceWrapper.ExecutionSegmentSequence[newIndex],
		SegmentIndex: newIndex,
	}, nil
}

// SegmentedIndex is an iterator that returns both the scaled and the unscaled
// sequential values according to the given ExecutionTuple. It is not
// thread-safe, concurrent access has to be externally synchronized.
//
// See the documentation for ExecutionTuple above for a visual explanation of
// how this iterator actually works.
type SegmentedIndex struct {
	start, lcd       int64
	offsets          []int64
	scaled, unscaled int64 // for both the first element(vu) is 1 not 0
}

// NewSegmentedIndex returns a pointer to a new SegmentedIndex instance,
// given an ExecutionTuple.
func NewSegmentedIndex(et *ExecutionTuple) *SegmentedIndex {
	start, offsets, lcd := et.GetStripedOffsets()
	return &SegmentedIndex{start: start, lcd: lcd, offsets: offsets}
}

// Next goes to the next scaled index and moves the unscaled one accordingly.
func (s *SegmentedIndex) Next() (int64, int64) {
	if s.scaled == 0 { // the 1 element(VU) is at the start
		s.unscaled += s.start + 1 // the first element of the start 0, but the here we need it to be 1 so we add 1
	} else { // if we are not at the first element we need to go through the offsets, looping over them
		s.unscaled += s.offsets[int(s.scaled-1)%len(s.offsets)] // slice's index start at 0 ours start at 1
	}
	s.scaled++

	return s.scaled, s.unscaled
}

// Prev goes to the previous scaled value and sets the unscaled one accordingly.
// Calling Prev when s.scaled == 0 is undefined.
func (s *SegmentedIndex) Prev() (int64, int64) {
	if s.scaled == 1 { // we are the first need to go to the 0th element which means we need to remove the start
		s.unscaled -= s.start + 1 // this could've been just settign to 0
	} else { // not at the first element - need to get the previously added offset so
		s.unscaled -= s.offsets[int(s.scaled-2)%len(s.offsets)] // slice's index start 0 our start at 1
	}
	s.scaled--

	return s.scaled, s.unscaled
}

// GoTo sets the scaled index to its biggest value for which the corresponding
// unscaled index is smaller or equal to value.
func (s *SegmentedIndex) GoTo(value int64) (int64, int64) { // TODO optimize
	var gi int64
	// Because of the cyclical nature of the striping algorithm (with a cycle
	// length of LCD, the least common denominator), when scaling large values
	// (i.e. many multiples of the LCD), we can quickly calculate how many times
	// the cycle repeats.
	wholeCycles := (value / s.lcd)
	// So we can set some approximate initial values quickly, since we also know
	// precisely how many scaled values there are per cycle length.
	s.scaled = wholeCycles * int64(len(s.offsets))
	s.unscaled = wholeCycles*s.lcd + s.start + 1 // our indexes are from 1 the start is from 0
	// Approach the final value using the slow algorithm with the step by step loop
	// TODO: this can be optimized by another array with size offsets that instead of the offsets
	// from the previous is the offset from either 0 or start
	i := s.start
	for ; i < value%s.lcd; gi, i = gi+1, i+s.offsets[gi] {
		s.scaled++
		s.unscaled += s.offsets[gi]
	}

	if gi > 0 { // there were more values after the wholecycles
		// the last offset actually shouldn't have been added
		s.unscaled -= s.offsets[gi-1]
	} else if s.scaled > 0 { // we didn't actually have more values after the wholecycles but we still had some
		// in this case the unscaled value needs to move back by the last offset as it would've been
		// the one to get it from the value it needs to be to it's current one
		s.unscaled -= s.offsets[len(s.offsets)-1]
	}

	if s.scaled == 0 {
		s.unscaled = 0 // we would've added the start and 1
	}

	return s.scaled, s.unscaled
}
