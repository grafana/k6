package lib

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringToES(t *testing.T, str string) *ExecutionSegment {
	es := new(ExecutionSegment)
	require.NoError(t, es.UnmarshalText([]byte(str)))
	return es
}

func TestExecutionSegmentEquals(t *testing.T) {
	t.Parallel()

	t.Run("nil segment to full", func(t *testing.T) {
		t.Parallel()
		var nilEs *ExecutionSegment
		fullEs := stringToES(t, "0:1")
		require.True(t, nilEs.Equal(fullEs))
		require.True(t, fullEs.Equal(nilEs))
	})

	t.Run("To it's self", func(t *testing.T) {
		t.Parallel()
		es := stringToES(t, "1/2:2/3")
		require.True(t, es.Equal(es))
	})
}

func TestExecutionSegmentNew(t *testing.T) {
	t.Parallel()
	t.Run("from is below zero", func(t *testing.T) {
		t.Parallel()
		_, err := NewExecutionSegment(big.NewRat(-1, 1), big.NewRat(1, 1))
		require.Error(t, err)
	})
	t.Run("to is more than 1", func(t *testing.T) {
		t.Parallel()
		_, err := NewExecutionSegment(big.NewRat(0, 1), big.NewRat(2, 1))
		require.Error(t, err)
	})
	t.Run("from is smaller than to", func(t *testing.T) {
		t.Parallel()
		_, err := NewExecutionSegment(big.NewRat(1, 2), big.NewRat(1, 3))
		require.Error(t, err)
	})

	t.Run("from is equal to 'to'", func(t *testing.T) {
		t.Parallel()
		_, err := NewExecutionSegment(big.NewRat(1, 2), big.NewRat(1, 2))
		require.Error(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		_, err := NewExecutionSegment(big.NewRat(0, 1), big.NewRat(1, 1))
		require.NoError(t, err)
	})
}

func TestExecutionSegmentUnmarshalText(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		input  string
		output *ExecutionSegment
		isErr  bool
	}{
		{input: "0:1", output: &ExecutionSegment{from: zeroRat, to: oneRat}},
		{input: "0.5:0.75", output: &ExecutionSegment{from: big.NewRat(1, 2), to: big.NewRat(3, 4)}},
		{input: "1/2:3/4", output: &ExecutionSegment{from: big.NewRat(1, 2), to: big.NewRat(3, 4)}},
		{input: "50%:75%", output: &ExecutionSegment{from: big.NewRat(1, 2), to: big.NewRat(3, 4)}},
		{input: "2/4:75%", output: &ExecutionSegment{from: big.NewRat(1, 2), to: big.NewRat(3, 4)}},
		{input: "75%", output: &ExecutionSegment{from: zeroRat, to: big.NewRat(3, 4)}},
		{input: "125%", isErr: true},
		{input: "1a5%", isErr: true},
		{input: "1a5", isErr: true},
		{input: "1a5%:2/3", isErr: true},
		{input: "125%:250%", isErr: true},
		{input: "55%:50%", isErr: true},
		// TODO add more strange or not so strange cases
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.input, func(t *testing.T) {
			t.Parallel()
			es := new(ExecutionSegment)
			err := es.UnmarshalText([]byte(testCase.input))
			if testCase.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, es.Equal(testCase.output))

			// see if unmarshalling a stringified segment gets you back the same segment
			err = es.UnmarshalText([]byte(es.String()))
			require.NoError(t, err)
			require.True(t, es.Equal(testCase.output))
		})
	}

	t.Run("Unmarshal nilSegment.String", func(t *testing.T) {
		var nilEs *ExecutionSegment
		nilEsStr := nilEs.String()
		require.Equal(t, "0:1", nilEsStr)

		es := new(ExecutionSegment)
		err := es.UnmarshalText([]byte(nilEsStr))
		require.NoError(t, err)
		require.True(t, es.Equal(nilEs))
	})
}

func TestExecutionSegmentSplit(t *testing.T) {
	t.Parallel()

	var nilEs *ExecutionSegment
	_, err := nilEs.Split(-1)
	require.Error(t, err)

	_, err = nilEs.Split(0)
	require.Error(t, err)

	segments, err := nilEs.Split(1)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, "0:1", segments[0].String())

	segments, err = nilEs.Split(2)
	require.NoError(t, err)
	require.Len(t, segments, 2)
	assert.Equal(t, "0:1/2", segments[0].String())
	assert.Equal(t, "1/2:1", segments[1].String())

	segments, err = nilEs.Split(3)
	require.NoError(t, err)
	require.Len(t, segments, 3)
	assert.Equal(t, "0:1/3", segments[0].String())
	assert.Equal(t, "1/3:2/3", segments[1].String())
	assert.Equal(t, "2/3:1", segments[2].String())

	secondQuarter, err := NewExecutionSegment(big.NewRat(1, 4), big.NewRat(2, 4))
	require.NoError(t, err)

	segments, err = secondQuarter.Split(1)
	require.NoError(t, err)
	require.Len(t, segments, 1)
	assert.Equal(t, "1/4:1/2", segments[0].String())

	segments, err = secondQuarter.Split(2)
	require.NoError(t, err)
	require.Len(t, segments, 2)
	assert.Equal(t, "1/4:3/8", segments[0].String())
	assert.Equal(t, "3/8:1/2", segments[1].String())

	segments, err = secondQuarter.Split(3)
	require.NoError(t, err)
	require.Len(t, segments, 3)
	assert.Equal(t, "1/4:1/3", segments[0].String())
	assert.Equal(t, "1/3:5/12", segments[1].String())
	assert.Equal(t, "5/12:1/2", segments[2].String())

	segments, err = secondQuarter.Split(4)
	require.NoError(t, err)
	require.Len(t, segments, 4)
	assert.Equal(t, "1/4:5/16", segments[0].String())
	assert.Equal(t, "5/16:3/8", segments[1].String())
	assert.Equal(t, "3/8:7/16", segments[2].String())
	assert.Equal(t, "7/16:1/2", segments[3].String())
}

func TestExecutionSegmentFailures(t *testing.T) {
	t.Parallel()
	es := new(ExecutionSegment)
	require.NoError(t, es.UnmarshalText([]byte("0:0.25")))
	require.Equal(t, int64(1), es.Scale(2))
	require.Equal(t, int64(1), es.Scale(3))

	require.NoError(t, es.UnmarshalText([]byte("0.25:0.5")))
	require.Equal(t, int64(0), es.Scale(2))
	require.Equal(t, int64(1), es.Scale(3))

	require.NoError(t, es.UnmarshalText([]byte("0.5:0.75")))
	require.Equal(t, int64(1), es.Scale(2))
	require.Equal(t, int64(0), es.Scale(3))

	require.NoError(t, es.UnmarshalText([]byte("0.75:1")))
	require.Equal(t, int64(0), es.Scale(2))
	require.Equal(t, int64(1), es.Scale(3))
}

func TestExecutionTupleScale(t *testing.T) {
	t.Parallel()
	es := new(ExecutionSegment)
	require.NoError(t, es.UnmarshalText([]byte("0.5")))
	et, err := NewExecutionTuple(es, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), et.ScaleInt64(2))
	require.Equal(t, int64(2), et.ScaleInt64(3))

	require.NoError(t, es.UnmarshalText([]byte("0.5:1.0")))
	et, err = NewExecutionTuple(es, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), et.ScaleInt64(2))
	require.Equal(t, int64(1), et.ScaleInt64(3))

	ess, err := NewExecutionSegmentSequenceFromString("0,0.5,1")
	require.NoError(t, err)
	require.NoError(t, es.UnmarshalText([]byte("0.5")))
	et, err = NewExecutionTuple(es, &ess)
	require.NoError(t, err)
	require.Equal(t, int64(1), et.ScaleInt64(2))
	require.Equal(t, int64(2), et.ScaleInt64(3))

	require.NoError(t, es.UnmarshalText([]byte("0.5:1.0")))
	et, err = NewExecutionTuple(es, &ess)
	require.NoError(t, err)
	require.Equal(t, int64(1), et.ScaleInt64(2))
	require.Equal(t, int64(1), et.ScaleInt64(3))
}

func TestBigScale(t *testing.T) {
	t.Parallel()
	es := new(ExecutionSegment)
	ess, err := NewExecutionSegmentSequenceFromString("0,7/20,7/10,1")
	require.NoError(t, err)
	require.NoError(t, es.UnmarshalText([]byte("0:7/20")))
	et, err := NewExecutionTuple(es, &ess)
	require.NoError(t, err)
	require.Equal(t, int64(18), et.ScaleInt64(50))
}

func TestExecutionSegmentCopyScaleRat(t *testing.T) {
	t.Parallel()
	es := new(ExecutionSegment)
	twoRat := big.NewRat(2, 1)
	threeRat := big.NewRat(3, 1)
	require.NoError(t, es.UnmarshalText([]byte("0.5")))
	require.Equal(t, oneRat, es.CopyScaleRat(twoRat))
	require.Equal(t, big.NewRat(3, 2), es.CopyScaleRat(threeRat))

	require.NoError(t, es.UnmarshalText([]byte("0.5:1.0")))
	require.Equal(t, oneRat, es.CopyScaleRat(twoRat))
	require.Equal(t, big.NewRat(3, 2), es.CopyScaleRat(threeRat))

	var nilEs *ExecutionSegment
	require.Equal(t, twoRat, nilEs.CopyScaleRat(twoRat))
	require.Equal(t, threeRat, nilEs.CopyScaleRat(threeRat))
}

func TestExecutionSegmentInPlaceScaleRat(t *testing.T) {
	t.Parallel()
	es := new(ExecutionSegment)
	twoRat := big.NewRat(2, 1)
	threeRat := big.NewRat(3, 1)
	threeSecondsRat := big.NewRat(3, 2)
	require.NoError(t, es.UnmarshalText([]byte("0.5")))
	require.Equal(t, oneRat, es.InPlaceScaleRat(twoRat))
	require.Equal(t, oneRat, twoRat)
	require.Equal(t, threeSecondsRat, es.InPlaceScaleRat(threeRat))
	require.Equal(t, threeSecondsRat, threeRat)

	es = stringToES(t, "0.5:1.0")
	twoRat = big.NewRat(2, 1)
	threeRat = big.NewRat(3, 1)
	require.Equal(t, oneRat, es.InPlaceScaleRat(twoRat))
	require.Equal(t, oneRat, twoRat)
	require.Equal(t, threeSecondsRat, es.InPlaceScaleRat(threeRat))
	require.Equal(t, threeSecondsRat, threeRat)

	var nilEs *ExecutionSegment
	twoRat = big.NewRat(2, 1)
	threeRat = big.NewRat(3, 1)
	require.Equal(t, big.NewRat(2, 1), nilEs.InPlaceScaleRat(twoRat))
	require.Equal(t, big.NewRat(2, 1), twoRat)
	require.Equal(t, big.NewRat(3, 1), nilEs.InPlaceScaleRat(threeRat))
	require.Equal(t, big.NewRat(3, 1), threeRat)
}

func TestExecutionSegmentSubSegment(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name              string
		base, sub, result *ExecutionSegment
	}{
		// TODO add more strange or not so strange cases
		{
			name:   "nil base",
			base:   (*ExecutionSegment)(nil),
			sub:    stringToES(t, "0.2:0.3"),
			result: stringToES(t, "0.2:0.3"),
		},

		{
			name:   "nil sub",
			base:   stringToES(t, "0.2:0.3"),
			sub:    (*ExecutionSegment)(nil),
			result: stringToES(t, "0.2:0.3"),
		},
		{
			name:   "doc example",
			base:   stringToES(t, "1/2:1"),
			sub:    stringToES(t, "0:1/2"),
			result: stringToES(t, "1/2:3/4"),
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, testCase.result, testCase.base.SubSegment(testCase.sub))
		})
	}
}

func TestSplitBadSegment(t *testing.T) {
	t.Parallel()
	es := &ExecutionSegment{from: oneRat, to: zeroRat}
	_, err := es.Split(5)
	require.Error(t, err)
}

func TestSegmentExecutionFloatLength(t *testing.T) {
	t.Parallel()
	t.Run("nil has 1.0", func(t *testing.T) {
		var nilEs *ExecutionSegment
		require.Equal(t, 1.0, nilEs.FloatLength())
	})

	testCases := []struct {
		es       *ExecutionSegment
		expected float64
	}{
		// TODO add more strange or not so strange cases
		{
			es:       stringToES(t, "1/2:1"),
			expected: 0.5,
		},
		{
			es:       stringToES(t, "1/3:1"),
			expected: 0.66666,
		},

		{
			es:       stringToES(t, "0:1/2"),
			expected: 0.5,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.es.String(), func(t *testing.T) {
			t.Parallel()
			require.InEpsilon(t, testCase.expected, testCase.es.FloatLength(), 0.001)
		})
	}
}

func TestExecutionSegmentSequences(t *testing.T) {
	t.Parallel()

	_, err := NewExecutionSegmentSequence(stringToES(t, "0:1/3"), stringToES(t, "1/2:1"))
	assert.Error(t, err)
}

func TestExecutionSegmentStringSequences(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		seq         string
		expSegments []string
		expError    bool
		canReverse  bool
		// TODO: checks for least common denominator and maybe striped partitioning
	}{
		{seq: "", expSegments: nil},
		{seq: "0.5", expError: true},
		{seq: "1,1", expError: true},
		{seq: "-0.5,1", expError: true},
		{seq: "1/2,1/2", expError: true},
		{seq: "1/2,1/3", expError: true},
		{seq: "0,1,1/2", expError: true},
		{seq: "0.5,1", expSegments: []string{"1/2:1"}},
		{seq: "1/2,1", expSegments: []string{"1/2:1"}, canReverse: true},
		{seq: "1/3,2/3", expSegments: []string{"1/3:2/3"}, canReverse: true},
		{seq: "0,1/3,2/3", expSegments: []string{"0:1/3", "1/3:2/3"}, canReverse: true},
		{seq: "0,1/3,2/3,1", expSegments: []string{"0:1/3", "1/3:2/3", "2/3:1"}, canReverse: true},
		{seq: "0.5,0.7", expSegments: []string{"1/2:7/10"}},
		{seq: "0.5,0.7,1", expSegments: []string{"1/2:7/10", "7/10:1"}},
		{seq: "0,1/13,2/13,1/3,1/2,3/4,1", expSegments: []string{
			"0:1/13", "1/13:2/13", "2/13:1/3", "1/3:1/2", "1/2:3/4", "3/4:1",
		}, canReverse: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.seq, func(t *testing.T) {
			t.Parallel()
			result, err := NewExecutionSegmentSequenceFromString(tc.seq)
			if tc.expError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, len(tc.expSegments), len(result))
			for i, expStrSeg := range tc.expSegments {
				expSeg, errl := NewExecutionSegmentFromString(expStrSeg)
				require.NoError(t, errl)
				assert.Truef(t, expSeg.Equal(result[i]), "Segment %d (%s) should be equal to %s", i, result[i], expSeg)
			}
			if tc.canReverse {
				assert.Equal(t, result.String(), tc.seq)
			}
		})
	}
}

// Return a randomly distributed sequence of n amount of
// execution segments whose length totals 1.
func generateRandomSequence(t testing.TB, n, m int64, r *rand.Rand) ExecutionSegmentSequence {
	var err error
	ess := ExecutionSegmentSequence(make([]*ExecutionSegment, n))
	numerators := make([]int64, n)
	var denominator int64
	for i := int64(0); i < n; i++ {
		numerators[i] = 1 + r.Int63n(m)
		denominator += numerators[i]
	}
	from := big.NewRat(0, 1)
	for i := int64(0); i < n; i++ {
		to := new(big.Rat).Add(big.NewRat(numerators[i], denominator), from)
		ess[i], err = NewExecutionSegment(from, to)
		require.NoError(t, err)
		from = to
	}

	return ess
}

// Ensure that the sum of scaling all execution segments in
// the same sequence with scaling factor M results in M itself.
func TestExecutionSegmentScaleConsistency(t *testing.T) {
	t.Parallel()

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	t.Logf("Random source seeded with %d\n", seed)

	const numTests = 10
	for i := 0; i < numTests; i++ {
		scale := rand.Int31n(99) + 2
		seq := generateRandomSequence(t, r.Int63n(9)+2, 100, r)

		t.Run(fmt.Sprintf("%d_%s", scale, seq), func(t *testing.T) {
			t.Parallel()
			var total int64
			for _, segment := range seq {
				total += segment.Scale(int64(scale))
			}
			assert.Equal(t, int64(scale), total)
		})
	}
}

// Ensure that the sum of scaling all execution segments in
// the same sequence with scaling factor M results in M itself.
func TestExecutionTupleScaleConsistency(t *testing.T) {
	t.Parallel()

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	t.Logf("Random source seeded with %d\n", seed)

	const numTests = 10
	for i := 0; i < numTests; i++ {
		scale := rand.Int31n(99) + 2
		seq := generateRandomSequence(t, r.Int63n(9)+2, 200, r)

		et, err := NewExecutionTuple(seq[0], &seq)
		require.NoError(t, err)
		t.Run(fmt.Sprintf("%d_%s", scale, seq), func(t *testing.T) {
			t.Parallel()
			var total int64
			for i, segment := range seq {
				assert.True(t, segment.Equal(et.Sequence.ExecutionSegmentSequence[i]))
				total += et.Sequence.ScaleInt64(i, int64(scale))
			}
			assert.Equal(t, int64(scale), total)
		})
	}
}

func TestExecutionSegmentScaleNoWobble(t *testing.T) {
	t.Parallel()

	requireSegmentScaleGreater := func(t *testing.T, et *ExecutionTuple) {
		var i, lastResult int64
		for i = 1; i < 1000; i++ {
			result := et.ScaleInt64(i)
			require.True(t, result >= lastResult, "%d<%d", result, lastResult)
			lastResult = result
		}
	}

	// Baseline full segment test
	t.Run("0:1", func(t *testing.T) {
		t.Parallel()
		et, err := NewExecutionTuple(nil, nil)
		require.NoError(t, err)
		requireSegmentScaleGreater(t, et)
	})

	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	t.Logf("Random source seeded with %d\n", seed)

	// Random segments
	const numTests = 10
	for i := 0; i < numTests; i++ {
		seq := generateRandomSequence(t, r.Int63n(9)+2, 100, r)

		es := seq[rand.Intn(len(seq))]

		et, err := NewExecutionTuple(seq[0], &seq)
		require.NoError(t, err)
		t.Run(es.String(), func(t *testing.T) {
			t.Parallel()
			requireSegmentScaleGreater(t, et)
		})
	}
}

func TestGetStripedOffsets(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		seq     string
		seg     string
		start   int64
		offsets []int64
		lcd     int64
	}{
		// full sequences
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.3", start: 0, offsets: []int64{4, 3, 3}, lcd: 10},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.3:0.5", start: 1, offsets: []int64{4, 6}, lcd: 10},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.5:0.6", start: 2, offsets: []int64{10}, lcd: 10},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.6:0.7", start: 3, offsets: []int64{10}, lcd: 10},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.8:0.9", start: 8, offsets: []int64{10}, lcd: 10},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0.9:1", start: 9, offsets: []int64{10}, lcd: 10},
		{seq: "0,0.2,0.5,0.6,0.7,0.8,0.9,1", seg: "0.9:1", start: 9, offsets: []int64{10}, lcd: 10},
		{seq: "0,0.2,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.2", start: 1, offsets: []int64{4, 6}, lcd: 10},
		{seq: "0,0.2,0.5,0.6,0.7,0.8,0.9,1", seg: "0.6:0.7", start: 3, offsets: []int64{10}, lcd: 10},
		// not full sequences
		{seq: "0,0.2,0.5", seg: "0:0.2", start: 3, offsets: []int64{6, 4}, lcd: 10},
		{seq: "0,0.2,0.5", seg: "0.2:0.5", start: 1, offsets: []int64{4, 2, 4}, lcd: 10},
		{seq: "0,2/5,4/5", seg: "0:2/5", start: 0, offsets: []int64{3, 2}, lcd: 5},
		{seq: "0,2/5,4/5", seg: "2/5:4/5", start: 1, offsets: []int64{3, 2}, lcd: 5},
		// no sequence
		{seg: "0:0.2", start: 1, offsets: []int64{5}, lcd: 5},
		{seg: "0:1/5", start: 1, offsets: []int64{5}, lcd: 5},
		{seg: "0:2/10", start: 1, offsets: []int64{5}, lcd: 5},
		{seg: "0:0.4", start: 1, offsets: []int64{2, 3}, lcd: 5},
		{seg: "0:2/5", start: 1, offsets: []int64{2, 3}, lcd: 5},
		{seg: "2/5:4/5", start: 1, offsets: []int64{3, 2}, lcd: 5},
		{seg: "0:4/10", start: 1, offsets: []int64{2, 3}, lcd: 5},
		{seg: "1/10:5/10", start: 1, offsets: []int64{2, 2, 4, 2}, lcd: 10},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("seq:%s;segment:%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()
			ess, err := NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)
			segment, err := NewExecutionSegmentFromString(tc.seg)
			require.NoError(t, err)
			et, err := NewExecutionTuple(segment, &ess)
			require.NoError(t, err)

			start, offsets, lcd := et.GetStripedOffsets()

			assert.Equal(t, tc.start, start)
			assert.Equal(t, tc.offsets, offsets)
			assert.Equal(t, tc.lcd, lcd)

			ess2, err := NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)
			assert.Equal(t, ess.String(), ess2.String())
		})
	}
}

func TestSequenceLCD(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		seq string
		lcd int64
	}{
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", lcd: 10},
		{seq: "0,0.1,0.5,0.6,0.7,0.8,0.9,1", lcd: 10},
		{seq: "0,0.2,0.5,0.6,0.7,0.8,0.9,1", lcd: 10},
		{seq: "0,1/3,5/6", lcd: 6},
		{seq: "0,1/3,4/7", lcd: 21},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("seq:%s", tc.seq), func(t *testing.T) {
			t.Parallel()
			ess, err := NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)
			require.Equal(t, tc.lcd, ess.LCD())
		})
	}
}

func BenchmarkGetStripedOffsets(b *testing.B) {
	lengths := [...]int64{10, 100}
	const seed = 777
	r := rand.New(rand.NewSource(seed))

	for _, length := range lengths {
		length := length
		b.Run(fmt.Sprintf("length%d,seed%d", length, seed), func(b *testing.B) {
			sequence := generateRandomSequence(b, length, 100, r)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				segment := sequence[int(r.Int63())%len(sequence)]
				et, err := NewExecutionTuple(segment, &sequence)
				require.NoError(b, err)
				_, _, _ = et.GetStripedOffsets()
			}
		})
	}
}

func BenchmarkGetStripedOffsetsEven(b *testing.B) {
	lengths := [...]int64{10, 100, 1000}
	generateSequence := func(n int64) ExecutionSegmentSequence {
		var err error
		ess := ExecutionSegmentSequence(make([]*ExecutionSegment, n))
		numerators := make([]int64, n)
		var denominator int64
		for i := int64(0); i < n; i++ {
			numerators[i] = 1 // nice and simple :)
			denominator += numerators[i]
		}
		ess[0], err = NewExecutionSegment(big.NewRat(0, 1), big.NewRat(numerators[0], denominator))
		require.NoError(b, err)
		for i := int64(1); i < n; i++ {
			ess[i], err = NewExecutionSegment(ess[i-1].to, new(big.Rat).Add(big.NewRat(numerators[i], denominator), ess[i-1].to))
			require.NoError(b, err, "%d", i)
		}

		return ess
	}

	for _, length := range lengths {
		length := length
		b.Run(fmt.Sprintf("length%d", length), func(b *testing.B) {
			sequence := generateSequence(length)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				segment := sequence[111233%len(sequence)]
				et, err := NewExecutionTuple(segment, &sequence)
				require.NoError(b, err)
				_, _, _ = et.GetStripedOffsets()
			}
		})
	}
}

func TestGetNewExecutionTupleBesedOnValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		seq      string
		seg      string
		value    int64
		expected string
	}{
		// full sequences
		{seq: "0,1/3,2/3,1", seg: "0:1/3", value: 20, expected: "0,7/20,7/10,1"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("seq:%s;segment:%s", tc.seq, tc.seg), func(t *testing.T) {
			t.Parallel()
			ess, err := NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(t, err)

			segment, err := NewExecutionSegmentFromString(tc.seg)
			require.NoError(t, err)

			et, err := NewExecutionTuple(segment, &ess)
			require.NoError(t, err)
			newET, err := et.GetNewExecutionTupleFromValue(tc.value)
			require.NoError(t, err)
			require.Equal(t, tc.expected, newET.Sequence.String())
		})
	}
}

func mustNewExecutionSegment(str string) *ExecutionSegment {
	res, err := NewExecutionSegmentFromString(str)
	if err != nil {
		panic(err)
	}
	return res
}

func mustNewExecutionSegmentSequence(str string) *ExecutionSegmentSequence {
	res, err := NewExecutionSegmentSequenceFromString(str)
	if err != nil {
		panic(err)
	}
	return &res
}

func TestNewExecutionTuple(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		seg           *ExecutionSegment
		seq           *ExecutionSegmentSequence
		scaleTests    map[int64]int64
		newScaleTests map[int64]map[int64]int64 // this is for after calling GetNewExecutionSegmentSequenceFromValue
	}{
		{
			// both segment and sequence are nil
			scaleTests: map[int64]int64{
				50: 50,
				1:  1,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 50, 1: 1, 0: 0},
				1:  {50: 50, 1: 1, 0: 0},
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("0:1"),
			// nil sequence
			scaleTests: map[int64]int64{
				50: 50,
				1:  1,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 50, 1: 1, 0: 0},
				1:  {50: 50, 1: 1, 0: 0},
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("0:1"),
			seq: mustNewExecutionSegmentSequence("0,1"),
			scaleTests: map[int64]int64{
				50: 50,
				1:  1,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 50, 1: 1, 0: 0},
				1:  {50: 50, 1: 1, 0: 0},
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("0:1"),
			seq: mustNewExecutionSegmentSequence(""),
			scaleTests: map[int64]int64{
				50: 50,
				1:  1,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 50, 1: 1, 0: 0},
				1:  {50: 50, 1: 1, 0: 0},
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("0:1/3"),
			seq: mustNewExecutionSegmentSequence("0,1/3,2/3,1"),
			scaleTests: map[int64]int64{
				50: 17,
				3:  1,
				2:  1,
				1:  1,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 17, 1: 1, 0: 0},
				20: {50: 18, 1: 1, 0: 0},
				3:  {50: 17, 1: 1, 0: 0},
				2:  {50: 25, 1: 1, 0: 0},
				1:  {50: 50, 1: 1, 0: 0},
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("1/3:2/3"),
			seq: mustNewExecutionSegmentSequence("0,1/3,2/3,1"),
			scaleTests: map[int64]int64{
				50: 17,
				3:  1,
				2:  1,
				1:  0,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 17, 1: 0, 0: 0},
				20: {50: 17, 1: 0, 0: 0},
				3:  {50: 17, 1: 0, 0: 0},
				2:  {50: 25, 1: 0, 0: 0},
				1:  nil,
				0:  nil,
			},
		},
		{
			seg: mustNewExecutionSegment("2/3:1"),
			seq: mustNewExecutionSegmentSequence("0,1/3,2/3,1"),
			scaleTests: map[int64]int64{
				50: 16,
				3:  1,
				2:  0,
				1:  0,
				0:  0,
			},
			newScaleTests: map[int64]map[int64]int64{
				50: {50: 16, 1: 0, 0: 0},
				20: {50: 15, 1: 0, 0: 0},
				3:  {50: 16, 1: 0, 0: 0},
				2:  nil,
				1:  nil,
				0:  nil,
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(fmt.Sprintf("seg:'%s',seq:'%s'", testCase.seg, testCase.seq), func(t *testing.T) {
			t.Parallel()
			et, err := NewExecutionTuple(testCase.seg, testCase.seq)
			require.NoError(t, err)

			for scaleValue, result := range testCase.scaleTests {
				require.Equal(t, result, et.ScaleInt64(scaleValue), "%d->%d", scaleValue, result)
			}

			for value, newResult := range testCase.newScaleTests {
				newET, err := et.GetNewExecutionTupleFromValue(value)
				if newResult == nil {
					require.Error(t, err)
					continue
				}
				require.NoError(t, err)
				for scaleValue, result := range newResult {
					require.Equal(t, result, newET.ScaleInt64(scaleValue),
						"GetNewExecutionTupleFromValue(%d)%d->%d", value, scaleValue, result)
				}
			}
		})
	}
}

func BenchmarkExecutionSegmentScale(b *testing.B) {
	testCases := []struct {
		seq string
		seg string
	}{
		{},
		{seg: "0:1"},
		{seq: "0,0.3,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.3"},
		{seq: "0,0.1,0.2,0.3,0.4,0.5,0.6,0.7,0.8,0.9,1", seg: "0:0.1"},
		{seg: "2/5:4/5"},
		{seg: "2235/5213:4/5"}, // just wanted it to be ugly ;D
	}

	for _, tc := range testCases {
		tc := tc
		b.Run(fmt.Sprintf("seq:%s;segment:%s", tc.seq, tc.seg), func(b *testing.B) {
			ess, err := NewExecutionSegmentSequenceFromString(tc.seq)
			require.NoError(b, err)
			segment, err := NewExecutionSegmentFromString(tc.seg)
			require.NoError(b, err)
			if tc.seg == "" {
				segment = nil // specifically for the optimization
			}
			et, err := NewExecutionTuple(segment, &ess)
			require.NoError(b, err)
			for _, value := range []int64{5, 5523, 5000000, 67280421310721} {
				value := value
				b.Run(fmt.Sprintf("segment.Scale(%d)", value), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						segment.Scale(value)
					}
				})

				b.Run(fmt.Sprintf("et.Scale(%d)", value), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						et, err = NewExecutionTuple(segment, &ess)
						require.NoError(b, err)
						et.ScaleInt64(value)
					}
				})

				et.ScaleInt64(1) // precache
				b.Run(fmt.Sprintf("et.Scale(%d) prefilled", value), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						et.ScaleInt64(value)
					}
				})
			}
		})
	}
}

// TODO: test with randomized things

func TestSegmentedIndex(t *testing.T) {
	t.Parallel()
	// TODO ... more structure ?
	t.Run("full", func(t *testing.T) {
		t.Parallel()
		s := SegmentedIndex{start: 0, lcd: 1, offsets: []int64{1}}

		s.Next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Next()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.Prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Prev()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)
	})

	t.Run("half", func(t *testing.T) {
		t.Parallel()
		s := SegmentedIndex{start: 0, lcd: 2, offsets: []int64{2}}

		s.Next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Next()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Next()
		assert.EqualValues(t, 5, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.Prev()
		assert.EqualValues(t, 3, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Prev()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 1, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)
	})

	t.Run("the other half", func(t *testing.T) {
		t.Parallel()
		s := SegmentedIndex{start: 1, lcd: 2, offsets: []int64{2}}

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Next()
		assert.EqualValues(t, 4, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Next()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.Prev()
		assert.EqualValues(t, 4, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)
	})

	t.Run("strange", func(t *testing.T) {
		t.Parallel()
		s := SegmentedIndex{start: 1, lcd: 7, offsets: []int64{4, 3}}

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Next()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Next()
		assert.EqualValues(t, 9, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.Prev()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)

		s.Next()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.GoTo(6)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.GoTo(5)
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.GoTo(7)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.GoTo(8)
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.GoTo(9)
		assert.EqualValues(t, 9, s.unscaled)
		assert.EqualValues(t, 3, s.scaled)

		s.Prev()
		assert.EqualValues(t, 6, s.unscaled)
		assert.EqualValues(t, 2, s.scaled)

		s.Prev()
		assert.EqualValues(t, 2, s.unscaled)
		assert.EqualValues(t, 1, s.scaled)

		s.Prev()
		assert.EqualValues(t, 0, s.unscaled)
		assert.EqualValues(t, 0, s.scaled)
	})
}
