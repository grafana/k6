/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package lib

import (
	"encoding"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"sync"
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
		return nil, fmt.Errorf("segment start value should be at least 0 but was %s", from.FloatString(2))
	}
	if from.Cmp(to) >= 0 {
		return nil, fmt.Errorf("segment start(%s) should be less than its end(%s)", from.FloatString(2), to.FloatString(2))
	}
	if to.Cmp(oneRat) > 0 {
		return nil, fmt.Errorf("segment end value shouldn't be more than 1 but was %s", to.FloatString(2))
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
//  `1/2:3/4`, `0.5:0.75`, `50%:75%`, and even `2/4:75%` should be (1/2, 3/4]
// And values without a colon are the end of a first segment:
//  `20%`, `0.2`,  and `1/5` should be converted to (0, 1/5]
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
		return nil, fmt.Errorf("the number of parts should be at least 1, %d received", numParts)
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
//    c.from = a.from + b.from * (a.to - a.from)
//    c.to = c.from + (b.to - b.from) * (a.to - a.from)
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
					"the start value %s of segment #%d should be equal to the end value of the previous one, but it is %s",
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

// lowest common denominator
// https://en.wikipedia.org/wiki/Least_common_multiple#Using_the_greatest_common_divisor
func (ess ExecutionSegmentSequence) lcd() int64 {
	var acc = ess[0].length.Denom().Int64()
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

type sortInterfaceWrapper struct { // TODO: rename ? delete ? and replace ?
	slice []struct { // TODO better name ? maybe  a type of it's own ?
		numerator     int64
		originalIndex int
	}
	lcd int64
}

func newWrapper(ess ExecutionSegmentSequence) sortInterfaceWrapper {
	var result = sortInterfaceWrapper{
		slice: make([]struct {
			numerator     int64
			originalIndex int
		}, len(ess)),
		lcd: ess.lcd(),
	}

	for i := range ess {
		result.slice[i].numerator = ess[i].length.Num().Int64() * (result.lcd / ess[i].length.Denom().Int64())
		result.slice[i].originalIndex = i
	}

	sort.SliceStable(result.slice, func(i, j int) bool {
		return result.slice[i].numerator > result.slice[j].numerator
	})
	return result
}

// Imagine you have a number of rational numbers which all add up to 1 (or less) and call them
// segments.
// If you want each to get proportional amount of anything you need to give them their numerator
// count of elements for each denominator amount from the original elements. So for 1/3 you give 1
// element for each 3 elements. For 3/5 - 3 elements for each 5.
// If you have for example a sequence of with element with length 3/5 and 1/3 in order to know how
// to distribute it accurately you need to get the LCD(lowest common denominitor) in this case
// between 3 and 5 this is 15 and then to transform the numbers to have the same, LCD equal,
// denominator. So 3/5 becomes 9/15 and 1/3 becomes 5/15. So now for each 15 elements 9 need to go
// to the 3/5, and 5 need to go to 1/3.
//
// We use the below algorithm to split elements between ExecutionSegments by using their length as
// the rational number. As we would like to get non sequential elements we try to get the maximum
// distance between them. That is the number of elements divided by the number of elements for any
// given segment, which concidently is the length of the segment reversed.
// The algorithm below does the following:
// 1. Goes through the elements from 0 to the lcd-1
// 2. For each of element goes through the segments and looks if the amount of already taken
// elements by the given segment multiplied by that segment length inverted is equal to or less to
// the current element index. if it is give that element to that segment if not continue with the
// next element.
//
// The code below specifically avoids using big.Rat which complicates the code somewhat.
// As additional note the sorting of the segments from biggest to smallest helps with the fact that
// the biggest elements will need to take the most elements and for them it will be the hardest to
// not get sequential elements.
func (e sortInterfaceWrapper) stripingAlgorithm(saveIndex func(iteration int64, index int, numerator int64) bool) {
	var chosenCounts = make([]int64, len(e.slice))

outer:
	for i := int64(0); i < e.lcd; i++ {
		for index, chosenCount := range chosenCounts {
			num := chosenCount * e.lcd
			denom := e.slice[index].numerator
			if i > num/denom || (i == num/denom && num%denom == 0) {
				chosenCounts[index]++
				if saveIndex(i, e.slice[index].originalIndex, denom) {
					break outer
				}
				break
			}
		}
	}
}

// ExecutionTuple is here to represent the combination of ExecutionSegmentSequence and
// ExecutionSegment and to give easy access to a couple of algorithms based on them in a way that is
// somewhat perfomant for which it generally needs to cache the results
type ExecutionTuple struct { // TODO rename
	ES *ExecutionSegment // TODO unexport this as well?

	esIndex      int
	sequence     ExecutionSegmentSequence
	offsetsCache [][]int64
	lcd          int64
	// TODO discuss if we just don't want to fillCache in the constructor and not need to use pointer receivers everywhere
	once *sync.Once
}

func fillSequence(sequence ExecutionSegmentSequence) ExecutionSegmentSequence {
	if sequence[0].from.Cmp(zeroRat) != 0 {
		es := newExecutionSegment(zeroRat, sequence[0].from)
		sequence = append(ExecutionSegmentSequence{es}, sequence...)
	}

	if sequence[len(sequence)-1].to.Cmp(oneRat) != 0 {
		es := newExecutionSegment(sequence[len(sequence)-1].to, oneRat)
		sequence = append(sequence, es)
	}
	return sequence
}

// NewExecutionTuple returns a new ExecutionTuple for the provided segment and sequence
func NewExecutionTuple(segment *ExecutionSegment, sequence *ExecutionSegmentSequence) (*ExecutionTuple, error) {
	et := ExecutionTuple{
		once: new(sync.Once),
		ES:   segment,
	}
	if sequence == nil || len(*sequence) == 0 {
		if segment == nil || segment.length.Cmp(oneRat) == 0 {
			// here we replace it with a not nil as we otherwise will need to check it everywhere
			et.sequence = ExecutionSegmentSequence{newExecutionSegment(zeroRat, oneRat)}
		} else {
			et.sequence = fillSequence(ExecutionSegmentSequence{segment})
		}
	} else {
		et.sequence = fillSequence(*sequence)
	}

	et.esIndex = et.find(segment)
	if et.esIndex == -1 {
		return nil, fmt.Errorf("couldn't find segment %s in sequence %s", segment, sequence)
	}
	return &et, nil
}

func (et *ExecutionTuple) find(segment *ExecutionSegment) int {
	if segment == nil {
		if len(et.sequence) == 1 {
			return 0
		}
		return -1
	}
	index := sort.Search(len(et.sequence), func(i int) bool {
		return et.sequence[i].from.Cmp(segment.from) >= 0
	})

	if index < 0 || index >= len(et.sequence) || !et.sequence[index].Equal(segment) {
		return -1
	}
	return index
}

// ScaleInt64 scales the provided value based on the ExecutionTuple
func (et *ExecutionTuple) ScaleInt64(value int64) int64 {
	if et.esIndex == -1 {
		return 0
	}
	if len(et.sequence) == 1 {
		return value
	}
	et.once.Do(et.fillCache)
	offsets := et.offsetsCache[et.esIndex]
	return scaleInt64(value, offsets[0], offsets[1:], et.lcd)
}

// scaleInt64With scales the provided value based on the ExecutionTuples'
// sequence and the segment provided
func (et *ExecutionTuple) scaleInt64With(value int64, es *ExecutionSegment) int64 { //nolint:unused
	start, offsets, lcd := et.GetStripedOffsets(es)
	return scaleInt64(value, start, offsets, lcd)
}

func scaleInt64(value, start int64, offsets []int64, lcd int64) int64 {
	endValue := (value / lcd) * int64(len(offsets))
	for gi, i := 0, start; i < value%lcd; gi, i = gi+1, i+offsets[gi] {
		endValue++
	}
	return endValue
}

func (et *ExecutionTuple) fillCache() {
	var wrapper = newWrapper(et.sequence)

	et.offsetsCache = make([][]int64, len(et.sequence))
	for i := range et.offsetsCache {
		et.offsetsCache[i] = make([]int64, 0, wrapper.slice[i].numerator)
	}

	var prev = make([]int64, len(et.sequence))
	var saveIndex = func(iteration int64, index int, numerator int64) bool {
		et.offsetsCache[index] = append(et.offsetsCache[index], iteration-prev[index])
		prev[index] = iteration
		if int64(len(et.offsetsCache[index])) == numerator {
			et.offsetsCache[index] = append(et.offsetsCache[index], et.offsetsCache[index][0]+wrapper.lcd-iteration)
		}
		return false
	}

	wrapper.stripingAlgorithm(saveIndex)
	et.lcd = wrapper.lcd
}

// GetStripedOffsets returns the stripped offsets for the given segment
// the returned values are as follows in order:
// - start: the first value that is for the segment
// - offsets: a list of offsets from the previous value for the segment. This are only the offsets
//            to from the start to the next start if we chunk the elements we are going to strip
//            into lcd sized chunks
// - lcd: the LCD of the lengths of all segments in the sequence. This is also the number of
//        elements after which the algorithm starts to loop and give the same values
func (et *ExecutionTuple) GetStripedOffsets(segment *ExecutionSegment) (int64, []int64, int64) {
	et.once.Do(et.fillCache)
	index := et.find(segment)
	if index == -1 {
		return -1, nil, et.lcd
	}
	offsets := et.offsetsCache[index]
	return offsets[0], offsets[1:], et.lcd
}

// GetNewExecutionTupleBasedOnValue uses the value provided, splits it using the striping offsets
// between all the segments in the sequence and returns a new ExecutionTuple with a new sequence and
// segments, such that each new segment in the new sequence has length `Scale(value)/value` while
// keeping the order. The main segment in the new ExecutionTuple is the correspoding one from the
// original, if that segmetn would've been with length 0 then it is nil, and obviously isn't part of
// the sequence.
func (et *ExecutionTuple) GetNewExecutionTupleBasedOnValue(value int64) *ExecutionTuple {
	var (
		newESS  = make(ExecutionSegmentSequence, 0, len(et.sequence)) // this can be smaller
		newES   *ExecutionSegment
		esIndex = -1
	)
	et.once.Do(et.fillCache)
	var prev int64
	for i := range et.sequence {
		offsets := et.offsetsCache[i]
		newValue := scaleInt64(value, offsets[0], offsets[1:], et.lcd)
		if newValue == 0 {
			continue
		}
		var currentES = newExecutionSegment(big.NewRat(prev, value), big.NewRat(prev+newValue, value))
		prev += newValue
		if i == et.esIndex {
			newES = currentES
			esIndex = len(newESS)
		}
		newESS = append(newESS, currentES)
	}
	return &ExecutionTuple{
		ES:       newES,
		sequence: newESS,
		esIndex:  esIndex,
		once:     new(sync.Once),
	}
}
