// Copyright 2016, Circonus, Inc. All rights reserved.
// See the LICENSE file.

// Package circllhist provides an implementation of Circonus' fixed log-linear
// histogram data structure.  This allows tracking of histograms in a
// composable way such that accurate error can be reasoned about.
package circonusllhist

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultHistSize = uint16(100)
)

var powerOfTen = [...]float64{
	1, 10, 100, 1000, 10000, 100000, 1e+06, 1e+07, 1e+08, 1e+09, 1e+10,
	1e+11, 1e+12, 1e+13, 1e+14, 1e+15, 1e+16, 1e+17, 1e+18, 1e+19, 1e+20,
	1e+21, 1e+22, 1e+23, 1e+24, 1e+25, 1e+26, 1e+27, 1e+28, 1e+29, 1e+30,
	1e+31, 1e+32, 1e+33, 1e+34, 1e+35, 1e+36, 1e+37, 1e+38, 1e+39, 1e+40,
	1e+41, 1e+42, 1e+43, 1e+44, 1e+45, 1e+46, 1e+47, 1e+48, 1e+49, 1e+50,
	1e+51, 1e+52, 1e+53, 1e+54, 1e+55, 1e+56, 1e+57, 1e+58, 1e+59, 1e+60,
	1e+61, 1e+62, 1e+63, 1e+64, 1e+65, 1e+66, 1e+67, 1e+68, 1e+69, 1e+70,
	1e+71, 1e+72, 1e+73, 1e+74, 1e+75, 1e+76, 1e+77, 1e+78, 1e+79, 1e+80,
	1e+81, 1e+82, 1e+83, 1e+84, 1e+85, 1e+86, 1e+87, 1e+88, 1e+89, 1e+90,
	1e+91, 1e+92, 1e+93, 1e+94, 1e+95, 1e+96, 1e+97, 1e+98, 1e+99, 1e+100,
	1e+101, 1e+102, 1e+103, 1e+104, 1e+105, 1e+106, 1e+107, 1e+108, 1e+109,
	1e+110, 1e+111, 1e+112, 1e+113, 1e+114, 1e+115, 1e+116, 1e+117, 1e+118,
	1e+119, 1e+120, 1e+121, 1e+122, 1e+123, 1e+124, 1e+125, 1e+126, 1e+127,
	1e-128, 1e-127, 1e-126, 1e-125, 1e-124, 1e-123, 1e-122, 1e-121, 1e-120,
	1e-119, 1e-118, 1e-117, 1e-116, 1e-115, 1e-114, 1e-113, 1e-112, 1e-111,
	1e-110, 1e-109, 1e-108, 1e-107, 1e-106, 1e-105, 1e-104, 1e-103, 1e-102,
	1e-101, 1e-100, 1e-99, 1e-98, 1e-97, 1e-96,
	1e-95, 1e-94, 1e-93, 1e-92, 1e-91, 1e-90, 1e-89, 1e-88, 1e-87, 1e-86,
	1e-85, 1e-84, 1e-83, 1e-82, 1e-81, 1e-80, 1e-79, 1e-78, 1e-77, 1e-76,
	1e-75, 1e-74, 1e-73, 1e-72, 1e-71, 1e-70, 1e-69, 1e-68, 1e-67, 1e-66,
	1e-65, 1e-64, 1e-63, 1e-62, 1e-61, 1e-60, 1e-59, 1e-58, 1e-57, 1e-56,
	1e-55, 1e-54, 1e-53, 1e-52, 1e-51, 1e-50, 1e-49, 1e-48, 1e-47, 1e-46,
	1e-45, 1e-44, 1e-43, 1e-42, 1e-41, 1e-40, 1e-39, 1e-38, 1e-37, 1e-36,
	1e-35, 1e-34, 1e-33, 1e-32, 1e-31, 1e-30, 1e-29, 1e-28, 1e-27, 1e-26,
	1e-25, 1e-24, 1e-23, 1e-22, 1e-21, 1e-20, 1e-19, 1e-18, 1e-17, 1e-16,
	1e-15, 1e-14, 1e-13, 1e-12, 1e-11, 1e-10, 1e-09, 1e-08, 1e-07, 1e-06,
	1e-05, 0.0001, 0.001, 0.01, 0.1,
}

// A Bracket is a part of a cumulative distribution.
type bin struct {
	count uint64
	val   int8
	exp   int8
}

func newBinRaw(val int8, exp int8, count uint64) *bin {
	return &bin{
		count: count,
		val:   val,
		exp:   exp,
	}
}

// func newBin() *bin {
// 	return newBinRaw(0, 0, 0)
// }

func newBinFromFloat64(d float64) *bin {
	hb := newBinRaw(0, 0, 0)
	hb.setFromFloat64(d)
	return hb
}

type fastL2 struct {
	l1, l2 int
}

func (hb *bin) newFastL2() fastL2 {
	return fastL2{l1: int(uint8(hb.exp)), l2: int(uint8(hb.val))}
}

func (hb *bin) setFromFloat64(d float64) *bin { //nolint:unparam
	hb.val = -1
	if math.IsInf(d, 0) || math.IsNaN(d) {
		return hb
	}
	if d == 0.0 {
		hb.val = 0
		return hb
	}
	sign := 1
	if math.Signbit(d) {
		sign = -1
	}
	d = math.Abs(d)
	bigExp := int(math.Floor(math.Log10(d)))
	hb.exp = int8(bigExp)
	if int(hb.exp) != bigExp { // rolled
		hb.exp = 0
		if bigExp < 0 {
			hb.val = 0
		}
		return hb
	}
	d /= hb.powerOfTen()
	d *= 10
	hb.val = int8(sign * int(math.Floor(d+1e-13)))
	if hb.val == 100 || hb.val == -100 {
		if hb.exp < 127 {
			hb.val /= 10
			hb.exp++
		} else {
			hb.val = 0
			hb.exp = 0
		}
	}
	if hb.val == 0 {
		hb.exp = 0
		return hb
	}
	if !((hb.val >= 10 && hb.val < 100) ||
		(hb.val <= -10 && hb.val > -100)) {
		hb.val = -1
		hb.exp = 0
	}
	return hb
}

func (hb *bin) powerOfTen() float64 {
	idx := int(uint8(hb.exp))
	return powerOfTen[idx]
}

func (hb *bin) isNaN() bool {
	// aval := abs(hb.val)
	aval := hb.val
	if aval < 0 {
		aval = -aval
	}
	if 99 < aval { // in [100... ]: nan
		return true
	}
	if 9 < aval { // in [10 - 99]: valid range
		return false
	}
	if 0 < aval { // in [1  - 9 ]: nan
		return true
	}
	if 0 == aval { // in [0]      : zero bucket
		return false
	}
	return false
}

func (hb *bin) value() float64 {
	if hb.isNaN() {
		return math.NaN()
	}
	if hb.val < 10 && hb.val > -10 {
		return 0.0
	}
	return (float64(hb.val) / 10.0) * hb.powerOfTen()
}

func (hb *bin) binWidth() float64 {
	if hb.isNaN() {
		return math.NaN()
	}
	if hb.val < 10 && hb.val > -10 {
		return 0.0
	}
	return hb.powerOfTen() / 10.0
}

func (hb *bin) midpoint() float64 {
	if hb.isNaN() {
		return math.NaN()
	}
	out := hb.value()
	if out == 0 {
		return 0
	}
	interval := hb.binWidth()
	if out < 0 {
		interval *= -1
	}
	return out + interval/2.0
}

func (hb *bin) left() float64 {
	if hb.isNaN() {
		return math.NaN()
	}
	out := hb.value()
	if out >= 0 {
		return out
	}
	return out - hb.binWidth()
}

func (hb *bin) compare(h2 *bin) int {
	var v1, v2 int

	// 1) slide exp positive
	// 2) shift by size of val multiple by (val != 0)
	// 3) then add or subtract val accordingly

	if hb.val >= 0 {
		v1 = ((int(hb.exp)+256)<<8)*(((int(hb.val)|(^int(hb.val)+1))>>8)&1) + int(hb.val)
	} else {
		v1 = ((int(hb.exp)+256)<<8)*(((int(hb.val)|(^int(hb.val)+1))>>8)&1) - int(hb.val)
	}

	if h2.val >= 0 {
		v2 = ((int(h2.exp)+256)<<8)*(((int(h2.val)|(^int(h2.val)+1))>>8)&1) + int(h2.val)
	} else {
		v2 = ((int(h2.exp)+256)<<8)*(((int(h2.val)|(^int(h2.val)+1))>>8)&1) - int(h2.val)
	}

	// return the difference
	return v2 - v1
}

// Histogram tracks values are two decimal digits of precision
// with a bounded error that remains bounded upon composition.
type Histogram struct {
	bvs       []bin
	lookup    [][]uint16
	mutex     sync.RWMutex
	used      uint16
	useLookup bool
	useLocks  bool
}

//nolint:golint,revive
const (
	BVL1, BVL1MASK uint64 = iota, 0xff << (8 * iota)
	BVL2, BVL2MASK
	BVL3, BVL3MASK
	BVL4, BVL4MASK
	BVL5, BVL5MASK
	BVL6, BVL6MASK
	BVL7, BVL7MASK
	BVL8, BVL8MASK
)

func getBytesRequired(val uint64) int8 {
	if 0 != (BVL8MASK|BVL7MASK|BVL6MASK|BVL5MASK)&val {
		if 0 != BVL8MASK&val {
			return int8(BVL8)
		}
		if 0 != BVL7MASK&val {
			return int8(BVL7)
		}
		if 0 != BVL6MASK&val {
			return int8(BVL6)
		}
		if 0 != BVL5MASK&val {
			return int8(BVL5)
		}
	} else {
		if 0 != BVL4MASK&val {
			return int8(BVL4)
		}
		if 0 != BVL3MASK&val {
			return int8(BVL3)
		}
		if 0 != BVL2MASK&val {
			return int8(BVL2)
		}
	}
	return int8(BVL1)
}

func writeBin(out io.Writer, in bin) (err error) {

	err = binary.Write(out, binary.BigEndian, in.val)
	if err != nil {
		return
	}

	err = binary.Write(out, binary.BigEndian, in.exp)
	if err != nil {
		return
	}

	var tgtType = getBytesRequired(in.count)

	err = binary.Write(out, binary.BigEndian, tgtType)
	if err != nil {
		return
	}

	var bcount = make([]uint8, 8)
	b := bcount[0 : tgtType+1]
	for i := tgtType; i >= 0; i-- {
		b[i] = uint8(uint64(in.count>>(uint8(i)*8)) & 0xff) //nolint:unconvert
	}

	err = binary.Write(out, binary.BigEndian, b)
	if err != nil {
		return
	}
	return
}

func readBin(in io.Reader) (bin, error) {
	var out bin

	err := binary.Read(in, binary.BigEndian, &out.val)
	if err != nil {
		return out, fmt.Errorf("read: %w", err)
	}

	err = binary.Read(in, binary.BigEndian, &out.exp)
	if err != nil {
		return out, fmt.Errorf("read: %w", err)
	}
	var bvl uint8
	err = binary.Read(in, binary.BigEndian, &bvl)
	if err != nil {
		return out, fmt.Errorf("read: %w", err)
	}
	if bvl > uint8(BVL8) {
		return out, fmt.Errorf("encoding error: bvl value is greater than max allowable") //nolint:goerr113
	}

	bcount := make([]byte, 8)
	b := bcount[0 : bvl+1]
	err = binary.Read(in, binary.BigEndian, b)
	if err != nil {
		return out, fmt.Errorf("read: %w", err)
	}

	count := uint64(0)
	for i := int(bvl + 1); i >= 0; i-- {
		count |= uint64(bcount[i]) << (uint8(i) * 8)
	}

	out.count = count
	return out, nil
}

func Deserialize(in io.Reader) (h *Histogram, err error) {
	return DeserializeWithOptions(in)
}

func DeserializeWithOptions(in io.Reader, options ...Option) (h *Histogram, err error) {
	var nbin int16
	err = binary.Read(in, binary.BigEndian, &nbin)
	if err != nil {
		return
	}

	options = append(options, Size(uint16(nbin)))
	h = New(options...)
	for ii := int16(0); ii < nbin; ii++ {
		bb, err := readBin(in)
		if err != nil {
			return h, err
		}
		h.insertBin(&bb, int64(bb.count))
	}
	return h, nil
}

func (h *Histogram) Serialize(w io.Writer) error {
	var nbin int16
	for i := range h.bvs {
		if h.bvs[i].count != 0 {
			nbin++
		}
	}

	if err := binary.Write(w, binary.BigEndian, nbin); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	for _, bv := range h.bvs {
		if bv.count != 0 {
			if err := writeBin(w, bv); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Histogram) SerializeB64(w io.Writer) error {
	buf := bytes.NewBuffer([]byte{})
	if err := h.Serialize(buf); err != nil {
		return err
	}

	encoder := base64.NewEncoder(base64.StdEncoding, w)
	if _, err := encoder.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("b64 encode write: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("b64 encoder close: %w", err)
	}

	return nil
}

// Options are exposed options for initializing a histogram.
type Options struct {
	// Size is the number of bins.
	Size uint16

	// UseLocks determines if the histogram should use locks
	UseLocks bool

	// UseLookup determines if the histogram should use a lookup table for bins
	UseLookup bool
}

// Option knows how to mutate the Options to change initialization.
type Option func(*Options)

// NoLocks configures a histogram to not use locks.
func NoLocks() Option {
	return func(options *Options) {
		options.UseLocks = false
	}
}

// NoLookup configures a histogram to not use a lookup table for bins.
// This is an appropriate option to use when the data set being operated
// over contains a large number of individual histograms and the insert
// speed into any histogram is not of the utmost importance. This option
// reduces the baseline memory consumption of one Histogram by at least
// 0.5kB and up to 130kB while increasing the insertion time by ~20%.
func NoLookup() Option {
	return func(options *Options) {
		options.UseLookup = false
	}
}

// Size configures a histogram to initialize a specific number of bins.
// When more bins are required, allocations increase linearly by the default
// size (100).
func Size(size uint16) Option {
	return func(options *Options) {
		options.Size = size
	}
}

// New returns a new Histogram, respecting the passed Options.
func New(options ...Option) *Histogram {
	o := Options{
		Size:      defaultHistSize,
		UseLocks:  true,
		UseLookup: true,
	}
	for _, opt := range options {
		opt(&o)
	}
	h := &Histogram{
		used:      0,
		bvs:       make([]bin, o.Size),
		useLocks:  o.UseLocks,
		useLookup: o.UseLookup,
	}
	if h.useLookup {
		h.lookup = make([][]uint16, 256)
	}
	return h
}

// NewNoLocks returns a new histogram not using locks.
// Deprecated: use New(NoLocks()) instead.
func NewNoLocks() *Histogram {
	return New(NoLocks())
}

// NewFromStrings returns a Histogram created from DecStrings strings.
func NewFromStrings(strs []string, locks bool) (*Histogram, error) {

	bin, err := stringsToBin(strs)
	if err != nil {
		return nil, err
	}

	return newFromBins(bin, locks), nil
}

// NewFromBins returns a Histogram created from a bins struct slice.
func newFromBins(bins []bin, locks bool) *Histogram {
	return &Histogram{
		used:      uint16(len(bins)),
		bvs:       bins,
		useLocks:  locks,
		lookup:    make([][]uint16, 256),
		useLookup: true,
	}
}

// Max returns the approximate maximum recorded value.
func (h *Histogram) Max() float64 {
	return h.ValueAtQuantile(1.0)
}

// Min returns the approximate minimum recorded value.
func (h *Histogram) Min() float64 {
	return h.ValueAtQuantile(0.0)
}

// Mean returns the approximate arithmetic mean of the recorded values.
func (h *Histogram) Mean() float64 {
	return h.ApproxMean()
}

// Count returns the number of recorded values.
func (h *Histogram) Count() uint64 {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	var count uint64
	for _, bin := range h.bvs[0:h.used] {
		if bin.isNaN() {
			continue
		}
		count += bin.count
	}
	return count
}

// BinCount returns the number of used bins.
func (h *Histogram) BinCount() uint64 {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	binCount := h.used
	return uint64(binCount)
}

// Reset forgets all bins in the histogram (they remain allocated).
func (h *Histogram) Reset() {
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}
	h.used = 0

	if !h.useLookup {
		return
	}
	for i := 0; i < 256; i++ {
		if h.lookup[i] != nil {
			for j := range h.lookup[i] {
				h.lookup[i][j] = 0
			}
		}
	}
}

// RecordIntScale records an integer scaler value, returning an error if the
// value is out of range.
func (h *Histogram) RecordIntScale(val int64, scale int) error {
	return h.RecordIntScales(val, scale, 1)
}

// RecordValue records the given value, returning an error if the value is out
// of range.
func (h *Histogram) RecordValue(v float64) error {
	return h.RecordValues(v, 1)
}

// RecordDuration records the given time.Duration in seconds, returning an error
// if the value is out of range.
func (h *Histogram) RecordDuration(v time.Duration) error {
	return h.RecordIntScale(int64(v), -9)
}

// RecordCorrectedValue records the given value, correcting for stalls in the
// recording process. This only works for processes which are recording values
// at an expected interval (e.g., doing jitter analysis). Processes which are
// recording ad-hoc values (e.g., latency for incoming requests) can't take
// advantage of this.
// CH Compat.
func (h *Histogram) RecordCorrectedValue(v, expectedInterval int64) error {
	if err := h.RecordValue(float64(v)); err != nil {
		return err
	}

	if expectedInterval <= 0 || v <= expectedInterval {
		return nil
	}

	missingValue := v - expectedInterval
	for missingValue >= expectedInterval {
		if err := h.RecordValue(float64(missingValue)); err != nil {
			return err
		}
		missingValue -= expectedInterval
	}

	return nil
}

// find where a new bin should go.
func (h *Histogram) internalFind(hb *bin) (bool, uint16) {
	if h.used == 0 {
		return false, 0
	}
	if h.useLookup {
		f2 := hb.newFastL2()
		if h.lookup[f2.l1] != nil {
			if idx := h.lookup[f2.l1][f2.l2]; idx != 0 {
				return true, idx - 1
			}
		}
	}
	rv := -1
	idx := uint16(0)
	l := int(0)
	r := int(h.used - 1)
	for l < r {
		check := (r + l) / 2
		rv = h.bvs[check].compare(hb)
		switch {
		case rv == 0:
			l = check
			r = check
		case rv > 0:
			l = check + 1
		default:
			r = check - 1
		}
	}
	if rv != 0 {
		rv = h.bvs[l].compare(hb)
	}
	idx = uint16(l)
	if rv == 0 {
		return true, idx
	}
	if rv < 0 {
		return false, idx
	}
	idx++
	return false, idx
}

func (h *Histogram) insertBin(hb *bin, count int64) uint64 { //nolint:unparam
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}
	found, idx := h.internalFind(hb)
	if !found {
		count := h.insertNewBinAt(idx, hb, count)
		// update the fast lookup table data after the index
		h.updateFast(idx)
		return count
	}
	return h.updateOldBinAt(idx, count)
}

func (h *Histogram) insertNewBinAt(idx uint16, hb *bin, count int64) uint64 {
	h.bvs = append(h.bvs, bin{})
	copy(h.bvs[idx+1:], h.bvs[idx:])
	h.bvs[idx].val = hb.val
	h.bvs[idx].exp = hb.exp
	h.bvs[idx].count = uint64(count)
	h.used++
	return h.bvs[idx].count
}

func (h *Histogram) updateFast(start uint16) {
	if !h.useLookup {
		return
	}
	for i := start; i < h.used; i++ {
		f2 := h.bvs[i].newFastL2()
		if h.lookup[f2.l1] == nil {
			h.lookup[f2.l1] = make([]uint16, 256)
		}
		h.lookup[f2.l1][f2.l2] = i + 1
	}
}

func (h *Histogram) updateOldBinAt(idx uint16, count int64) uint64 {
	var newval uint64
	if count >= 0 {
		newval = h.bvs[idx].count + uint64(count)
	} else {
		newval = h.bvs[idx].count - uint64(-count)
	}
	if newval < h.bvs[idx].count { // rolled
		newval = ^uint64(0)
	}
	h.bvs[idx].count = newval
	return newval - h.bvs[idx].count
}

// RecordIntScales records n occurrences of the given value, returning an error if
// the value is out of range.
func (h *Histogram) RecordIntScales(val int64, scale int, n int64) error {
	sign := int64(1)
	if val == 0 {
		scale = 0
	} else {
		scale++
		if val < 0 {
			val = 0 - val
			sign = -1
		}
		if val < 10 {
			val *= 10
			scale--
		}
		for val >= 100 {
			val /= 10
			scale++
		}
	}
	if scale < -128 {
		val = 0
		scale = 0
	} else if scale > 127 {
		val = 0xff
		scale = 0
	}
	val *= sign
	hb := bin{val: int8(val), exp: int8(scale), count: 0}
	h.insertBin(&hb, n)
	return nil
}

// RecordValues records n occurrences of the given value, returning an error if
// the value is out of range.
func (h *Histogram) RecordValues(v float64, n int64) error {
	var hb bin
	hb.setFromFloat64(v)
	h.insertBin(&hb, n)
	return nil
}

// ApproxMean returns an approximate mean.
func (h *Histogram) ApproxMean() float64 {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	divisor := 0.0
	sum := 0.0
	for i := uint16(0); i < h.used; i++ {
		midpoint := h.bvs[i].midpoint()
		cardinality := float64(h.bvs[i].count)
		divisor += cardinality
		sum += midpoint * cardinality
	}
	if divisor == 0.0 {
		return math.NaN()
	}
	return sum / divisor
}

// ApproxSum returns an approximate sum.
func (h *Histogram) ApproxSum() float64 {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	sum := 0.0
	for i := uint16(0); i < h.used; i++ {
		midpoint := h.bvs[i].midpoint()
		cardinality := float64(h.bvs[i].count)
		sum += midpoint * cardinality
	}
	return sum
}

func (h *Histogram) ApproxQuantile(qIn []float64) ([]float64, error) {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	qOut := make([]float64, len(qIn))
	iq, ib := 0, uint16(0)
	totalCnt, binWidth, binLeft, lowerCnt, upperCnt := 0.0, 0.0, 0.0, 0.0, 0.0
	if len(qIn) == 0 {
		return qOut, nil
	}
	// Make sure the requested quantiles are in order
	for iq = 1; iq < len(qIn); iq++ {
		if qIn[iq-1] > qIn[iq] {
			return nil, fmt.Errorf("out of order") //nolint:goerr113
		}
	}
	// Add up the bins
	for ib = 0; ib < h.used; ib++ {
		if !h.bvs[ib].isNaN() {
			totalCnt += float64(h.bvs[ib].count)
		}
	}
	if totalCnt == 0.0 {
		return nil, fmt.Errorf("empty_histogram") //nolint:goerr113
	}

	for iq = 0; iq < len(qIn); iq++ {
		if qIn[iq] < 0.0 || qIn[iq] > 1.0 {
			return nil, fmt.Errorf("out of bound quantile") //nolint:goerr113
		}
		qOut[iq] = totalCnt * qIn[iq]
	}

	for ib = 0; ib < h.used; ib++ {
		if h.bvs[ib].isNaN() {
			continue
		}
		binWidth = h.bvs[ib].binWidth()
		binLeft = h.bvs[ib].left()
		lowerCnt = upperCnt
		upperCnt = lowerCnt + float64(h.bvs[ib].count)
		break
	}
	for iq = 0; iq < len(qIn); iq++ {
		for ib < (h.used-1) && upperCnt < qOut[iq] {
			ib++
			binWidth = h.bvs[ib].binWidth()
			binLeft = h.bvs[ib].left()
			lowerCnt = upperCnt
			upperCnt = lowerCnt + float64(h.bvs[ib].count)
		}
		switch {
		case lowerCnt == qOut[iq]:
			qOut[iq] = binLeft
		case upperCnt == qOut[iq]:
			qOut[iq] = binLeft + binWidth
		default:
			if binWidth == 0 {
				qOut[iq] = binLeft
			} else {
				qOut[iq] = binLeft + (qOut[iq]-lowerCnt)/(upperCnt-lowerCnt)*binWidth
			}
		}
	}
	return qOut, nil
}

// ValueAtQuantile returns the recorded value at the given quantile (0..1).
func (h *Histogram) ValueAtQuantile(q float64) float64 {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	qIn := make([]float64, 1)
	qIn[0] = q
	qOut, err := h.ApproxQuantile(qIn)
	if err == nil && len(qOut) == 1 {
		return qOut[0]
	}
	return math.NaN()
}

// SignificantFigures returns the significant figures used to create the
// histogram
// CH Compat.
func (h *Histogram) SignificantFigures() int64 {
	return 2
}

// Equals returns true if the two Histograms are equivalent, false if not.
func (h *Histogram) Equals(other *Histogram) bool {
	if h.useLocks {
		h.mutex.RLock()
		defer h.mutex.RUnlock()
	}
	if other.useLocks {
		other.mutex.RLock()
		defer other.mutex.RUnlock()
	}
	switch {
	case
		h.used != other.used:
		return false
	default:
		for i := uint16(0); i < h.used; i++ {
			if h.bvs[i].compare(&other.bvs[i]) != 0 {
				return false
			}
			if h.bvs[i].count != other.bvs[i].count {
				return false
			}
		}
	}
	return true
}

// Copy creates and returns an exact copy of a histogram.
func (h *Histogram) Copy() *Histogram {
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	newhist := New()
	newhist.used = h.used
	newhist.useLocks = h.useLocks

	newhist.bvs = make([]bin, len(h.bvs))
	copy(h.bvs, newhist.bvs)

	newhist.useLookup = h.useLookup
	if h.useLookup {
		newhist.lookup = make([][]uint16, 256)
		for i, u := range h.lookup {
			newhist.lookup[i] = append(newhist.lookup[i], u...)
		}
	}

	return newhist
}

// FullReset resets a histogram to default empty values.
func (h *Histogram) FullReset() {
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	h.bvs = []bin{}
	h.used = 0
	if h.useLookup {
		h.lookup = make([][]uint16, 256)
	}
}

// CopyAndReset creates and returns an exact copy of a histogram,
// and resets it to default empty values.
func (h *Histogram) CopyAndReset() *Histogram {
	newhist := h.Copy()
	h.FullReset()
	return newhist
}

func (h *Histogram) DecStrings() []string {
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}
	out := make([]string, h.used)
	for i, bin := range h.bvs[0:h.used] {
		var buffer bytes.Buffer
		buffer.WriteString("H[")
		buffer.WriteString(fmt.Sprintf("%3.1e", bin.value()))
		buffer.WriteString("]=")
		buffer.WriteString(fmt.Sprintf("%v", bin.count))
		out[i] = buffer.String()
	}
	return out
}

// takes the output of DecStrings and deserializes it into a Bin struct slice.
func stringsToBin(strs []string) ([]bin, error) {

	bins := make([]bin, len(strs))
	for i, str := range strs {

		// H[0.0e+00]=1

		// H[0.0e+00]= <1>
		countString := strings.Split(str, "=")[1]
		countInt, err := strconv.ParseInt(countString, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse int: %w", err)
		}

		// H[ <0.0> e+00]=1
		valString := strings.Split(strings.Split(strings.Split(str, "=")[0], "e")[0], "[")[1]
		valInt, err := strconv.ParseFloat(valString, 64)
		if err != nil {
			return nil, fmt.Errorf("parse float: %w", err)
		}

		// H[0.0e <+00> ]=1
		expString := strings.Split(strings.Split(strings.Split(str, "=")[0], "e")[1], "]")[0]
		expInt, err := strconv.ParseInt(expString, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("parse int: %w", err)
		}
		bins[i] = *newBinRaw(int8(valInt*10), int8(expInt), uint64(countInt))
	}

	return bins, nil
}

// UnmarshalJSON - histogram will come in a base64 encoded serialized form.
func (h *Histogram) UnmarshalJSON(b []byte) error {
	return UnmarshalJSONWithOptions(h, b)
}

// UnmarshalJSONWithOptions unmarshals the byte data into the parent histogram,
// using the provided Options to create the output Histogram.
func UnmarshalJSONWithOptions(parent *Histogram, b []byte, options ...Option) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("b64 decode: %w", err)
	}

	hNew, err := DeserializeWithOptions(bytes.NewBuffer(data), options...)
	if err != nil {
		return err
	}

	// Go's JSON package will create a new Histogram to deserialize into by
	// reflection, so all fields will have their zero values. Some of the
	// default Histogram fields are not the zero values, so we can set them
	// by proxy from the new histogram that's been created from deserialization.
	parent.useLocks = hNew.useLocks
	parent.useLookup = hNew.useLookup
	if parent.useLookup {
		parent.lookup = make([][]uint16, 256)
	}

	parent.Merge(hNew)
	return nil
}

func (h *Histogram) MarshalJSON() ([]byte, error) {
	return MarshalJSON(h)
}

func MarshalJSON(h *Histogram) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	err := h.SerializeB64(buf)
	if err != nil {
		return buf.Bytes(), err
	}
	data, err := json.Marshal(buf.String())
	if err != nil {
		return nil, fmt.Errorf("json marshal: %w", err)
	}
	return data, nil
}

// Merge merges all bins from another histogram.
func (h *Histogram) Merge(o *Histogram) {
	if o == nil {
		return
	}

	if o.useLocks {
		o.mutex.Lock()
		defer o.mutex.Unlock()
	}
	if h.useLocks {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	var i, j uint16
	for ; i < h.used && j < o.used; i++ {
		diff := h.bvs[i].compare(&o.bvs[j])
		// o.bvs[j] > h.bvs[i], do nothing.
		if diff > 0 {
			continue
		}

		b := &o.bvs[j]
		j++
		switch {
		case diff == 0:
			h.updateOldBinAt(i, int64(b.count))
		case diff < 0:
			h.insertNewBinAt(i, b, int64(b.count))
		}
	}

	// append the rest bins
	for ; j < o.used; j++ {
		h.insertNewBinAt(h.used, &o.bvs[j], int64(o.bvs[j].count))
	}

	// rebuild all the fast lookup table
	h.updateFast(0)
}

// HistogramWithoutLookups holds a Histogram that's not configured to use
// a lookup table. This type is useful to round-trip serialize the underlying
// data while never allocating memory for the lookup table.
// The main Histogram type must use lookups by default to be compatible with
// the circllhist implementation of other languages. Furthermore, it is not
// possible to encode the lookup table preference into the serialized form,
// as that's again defined across languages. Therefore, the most straightforward
// manner by which a user can deserialize histogram data while not allocating
// lookup tables is by using a dedicated type in their structures describing
// on-disk forms.
// This structure can divulge the underlying Histogram, optionally allocating
// the lookup tables first.
type HistogramWithoutLookups struct {
	histogram *Histogram
}

// NewHistogramWithoutLookups creates a new container for a Histogram without
// lookup tables.
func NewHistogramWithoutLookups(histogram *Histogram) *HistogramWithoutLookups {
	histogram.useLookup = false
	histogram.lookup = nil
	return &HistogramWithoutLookups{
		histogram: histogram,
	}
}

// Histogram divulges the underlying Histogram that was deserialized. This
// Histogram will not have lookup tables allocated.
func (h *HistogramWithoutLookups) Histogram() *Histogram {
	return h.histogram
}

// HistogramWithLookups allocates lookup tables in the underlying Histogram that was
// deserialized, then divulges it.
func (h *HistogramWithoutLookups) HistogramWithLookups() *Histogram {
	h.histogram.useLookup = true
	h.histogram.lookup = make([][]uint16, 256)
	return h.histogram
}

// UnmarshalJSON unmarshals a histogram from a base64 encoded serialized form.
func (h *HistogramWithoutLookups) UnmarshalJSON(b []byte) error {
	var histogram Histogram
	if err := UnmarshalJSONWithOptions(&histogram, b, NoLookup()); err != nil {
		return err
	}
	h.histogram = &histogram
	return nil
}

// MarshalJSON marshals a histogram to a base64 encoded serialized form.
func (h *HistogramWithoutLookups) MarshalJSON() ([]byte, error) {
	return MarshalJSON(h.histogram)
}
