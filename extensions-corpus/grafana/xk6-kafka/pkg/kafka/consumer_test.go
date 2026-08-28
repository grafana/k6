package kafka

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func TestStartOffsetReset(t *testing.T) {
	t.Parallel()
	require.Equal(t, kgo.NewOffset().AtStart(), startOffsetReset(startOffsetFirst))
	require.Equal(t, kgo.NewOffset().AtStart(), startOffsetReset(""))
	require.Equal(t, kgo.NewOffset().AtEnd(), startOffsetReset(startOffsetLast))
}

func TestDirectOffset(t *testing.T) {
	t.Parallel()
	require.Equal(t, kgo.NewOffset().AtStart(), directOffset(0))
	require.Equal(t, kgo.NewOffset().AtEnd(), directOffset(-1))
	require.Equal(t, kgo.NewOffset().At(5), directOffset(5))
}

func TestIsolationLevel(t *testing.T) {
	t.Parallel()
	require.Equal(t, kgo.ReadCommitted(), isolationLevel(isolationReadCommitted))
	require.Equal(t, kgo.ReadUncommitted(), isolationLevel(isolationReadUncommitted))
	require.Equal(t, kgo.ReadUncommitted(), isolationLevel(""))
}

func TestGroupBalancers(t *testing.T) {
	t.Parallel()
	require.Len(t, groupBalancers([]string{groupBalancerRange, groupBalancerRoundRobin}), 2)
	// Rack affinity has no franz-go equivalent → ignored (caller defaults to range).
	require.Empty(t, groupBalancers([]string{groupBalancerRackAffinity}))
	require.Empty(t, groupBalancers([]string{"bogus"}))
	require.Empty(t, groupBalancers(nil))
}

func TestEffectiveGroupBalancers(t *testing.T) {
	t.Parallel()
	// Unset → range default (not franz-go's cooperative-sticky).
	require.Equal(t, "range", effectiveGroupBalancers(nil)[0].ProtocolName())
	// Only the ignored rack-affinity → range default.
	require.Equal(t, "range", effectiveGroupBalancers([]string{groupBalancerRackAffinity})[0].ProtocolName())
	// Explicit balancer is honored.
	bs := effectiveGroupBalancers([]string{groupBalancerRoundRobin})
	require.Len(t, bs, 1)
	require.Equal(t, "roundrobin", bs[0].ProtocolName())
}

func TestDecodeRecord(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	rec := &kgo.Record{
		Topic:     "t",
		Partition: 2,
		Offset:    42,
		Key:       []byte("k"),
		Value:     []byte("v"),
		Headers:   []kgo.RecordHeader{{Key: "h", Value: []byte("hv")}},
		Timestamp: ts,
	}

	m := decodeRecord(rec, 100, false)
	require.Equal(t, "t", m.Topic)
	require.Equal(t, 2, m.Partition)
	require.Equal(t, int64(42), m.Offset)
	require.Equal(t, int64(100), m.HighWaterMark)
	require.Equal(t, []byte("k"), m.Key)
	require.Equal(t, []byte("v"), m.Value)
	require.Equal(t, []byte("hv"), m.Headers["h"])
	require.Equal(t, ts.Format(time.RFC3339), m.Time)
	require.NotContains(t, m.Time, "123456789")

	// nanoPrecision keeps sub-second digits.
	mn := decodeRecord(rec, 100, true)
	require.Contains(t, mn.Time, "123456789")
}

func TestReaderLag(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(4), readerLag(10, 5))  // 10-5-1
	require.Equal(t, int64(0), readerLag(10, 9))  // at head: 10-9-1=0
	require.Equal(t, int64(0), readerLag(10, 10)) // offset==hwm: floored at 0
	require.Equal(t, int64(0), readerLag(0, 0))
}

func TestOpenReaderValidation(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()

	// No brokers.
	_, err := openReader(vu, ReaderConfig{Topic: "t"}, nil)
	require.Error(t, err)

	// Group with neither groupTopics nor topic.
	_, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, GroupID: "g"}, nil)
	require.Error(t, err)

	// Direct without topic.
	_, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}}, nil)
	require.Error(t, err)

	// Invalid maxWait.
	_, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t", MaxWait: "soon"}, nil)
	require.Error(t, err)

	// Offset below -1 is rejected.
	bad := int64(-2)
	_, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t", Offset: &bad}, nil)
	require.Error(t, err)

	// Negative maxAttempts is rejected.
	neg := -1
	_, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t", MaxAttempts: &neg}, nil)
	require.Error(t, err)

	// Valid direct and group constructions (NewClient is lazy).
	r, err := openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	r.Close()
	r, err = openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, GroupID: "g", Topic: "t"}, nil)
	require.NoError(t, err)
	r.Close()
}

func TestReaderExposesMethods(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	r, err := openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	t.Cleanup(r.Close)
	require.NoError(t, vu.Runtime().Set("r", r))

	v, err := vu.Runtime().RunString(`typeof r.consume === "function" && typeof r.close === "function"`)
	require.NoError(t, err)
	require.True(t, v.ToBoolean())
}

func TestConsumeRejectsInitContext(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	vu.Phase = extensionapi.ExecutionPhaseInit
	r, err := openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	t.Cleanup(r.Close)

	_, err = r.Consume(ConsumeConfig{Limit: 1})
	require.Error(t, err)
}

func TestConsumeCancellationNotTimeout(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	ctx, cancel := context.WithCancel(context.Background())
	vu.ContextValue = ctx
	r, err := openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	t.Cleanup(r.Close)

	cancel()
	// Even with expectTimeout, a canceled VU context surfaces as cancellation.
	_, err = r.Consume(ConsumeConfig{Limit: 1, ExpectTimeout: true})
	require.ErrorContains(t, err, "canceled")
}

func TestConsumeAfterCloseErrors(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	r, err := openReader(vu, ReaderConfig{Brokers: []string{"b:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	r.Close()

	_, err = r.Consume(ConsumeConfig{Limit: 1})
	require.Error(t, err)
}
