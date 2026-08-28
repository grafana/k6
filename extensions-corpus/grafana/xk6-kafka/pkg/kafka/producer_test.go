package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func TestCompressionCodec(t *testing.T) {
	t.Parallel()
	for _, name := range []string{codecGzip, codecSnappy, codecLz4, codecZstd} {
		_, ok := compressionCodec(name)
		require.True(t, ok, name)
	}
	_, ok := compressionCodec("")
	require.False(t, ok)
	_, ok = compressionCodec("bogus")
	require.False(t, ok)
}

func TestPartitioner(t *testing.T) {
	t.Parallel()
	// Named balancers map to a partitioner.
	for _, name := range []string{balancerRoundRobin, balancerLeastBytes, balancerHash, balancerMurmur2} {
		require.NotNil(t, partitioner(name), name)
	}
	// CRC32, custom function, and unset fall back to the default (nil).
	require.Nil(t, partitioner(balancerCrc32))
	require.Nil(t, partitioner(func() {}))
	require.Nil(t, partitioner(nil))
}

func TestAcksFromInt(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]kgo.Acks{-1: kgo.AllISRAcks(), 0: kgo.NoAck(), 1: kgo.LeaderAck()} {
		got, err := acksFromInt(n)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	// Out-of-range values are rejected.
	_, err := acksFromInt(2)
	require.Error(t, err)
	_, err = acksFromInt(99)
	require.Error(t, err)
}

func TestMarshalRecord(t *testing.T) {
	t.Parallel()

	now := time.Now()
	r := marshalRecord(&ProduceMessage{
		Topic:   "t",
		Key:     "k",
		Value:   []byte{1, 2, 3},
		Headers: map[string]any{"h": "v"},
		Time:    now,
	})
	require.Equal(t, "t", r.Topic)
	require.Equal(t, []byte("k"), r.Key)
	require.Equal(t, []byte{1, 2, 3}, r.Value)
	require.Equal(t, now, r.Timestamp)
	require.Len(t, r.Headers, 1)
	require.Equal(t, "h", r.Headers[0].Key)
	require.Equal(t, []byte("v"), r.Headers[0].Value)

	// Empty topic falls back to the writer default (left blank on the record).
	r = marshalRecord(&ProduceMessage{Value: "v"})
	require.Equal(t, "", r.Topic)
	require.Nil(t, r.Key)
	require.Equal(t, []byte("v"), r.Value)
	require.True(t, r.Timestamp.IsZero())
}

func TestToBytes(t *testing.T) {
	t.Parallel()
	require.Nil(t, toBytes(nil))
	require.Equal(t, []byte("s"), toBytes("s"))
	require.Equal(t, []byte{1, 2}, toBytes([]byte{1, 2}))
	require.Equal(t, []byte("42"), toBytes(42))
}

func TestOpenWriterRequiresBrokers(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	_, err := openWriter(vu, WriterConfig{Topic: "t"}, nil)
	require.Error(t, err)
}

func TestOpenWriterRejectsInvalidAcks(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	acks := 2
	_, err := openWriter(vu, WriterConfig{Brokers: []string{"localhost:9092"}, RequiredAcks: &acks}, nil)
	require.Error(t, err)
}

func TestOpenWriterRejectsNegativeMaxAttempts(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	n := -1
	_, err := openWriter(vu, WriterConfig{Brokers: []string{"localhost:9092"}, MaxAttempts: &n}, nil)
	require.Error(t, err)
}

func TestProduceAfterCloseErrors(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	w, err := openWriter(vu, WriterConfig{Brokers: []string{"localhost:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	w.Close()

	// Producing after close returns an error rather than panicking.
	err = w.Produce(ProduceConfig{Messages: []ProduceMessage{{Value: "v"}}})
	require.Error(t, err)
}

func TestWriterExposesMethods(t *testing.T) {
	t.Parallel()
	vu := extensionapitest.NewVU()
	// NewClient is lazy, so a Writer constructs without a broker.
	w, err := openWriter(vu, WriterConfig{Brokers: []string{"localhost:9092"}, Topic: "t"}, nil)
	require.NoError(t, err)
	t.Cleanup(w.Close)
	require.NoError(t, vu.Runtime().Set("w", w))

	v, err := vu.Runtime().RunString(`typeof w.produce === "function" && typeof w.close === "function"`)
	require.NoError(t, err)
	require.True(t, v.ToBoolean())
}
