package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
)

// newTestConnection builds a Connection with a lazy client (no dial) in the VU
// context, for exercising the admin methods' local logic without a broker.
func newTestConnection(t *testing.T) *Connection {
	t.Helper()
	rt := modulestest.NewRuntime(t)
	rt.MoveToVUContext(&lib.State{})
	client, err := kgo.NewClient(kgo.SeedBrokers("b:9092"))
	require.NoError(t, err)
	conn := &Connection{vu: rt.VU, client: client}
	t.Cleanup(conn.Close)
	return conn
}

func TestPartitionsAndReplication(t *testing.T) {
	t.Parallel()

	p, r, err := partitionsAndReplication(TopicConfig{})
	require.NoError(t, err)
	require.Equal(t, int32(1), p)
	require.Equal(t, int16(1), r)

	p, r, err = partitionsAndReplication(TopicConfig{NumPartitions: 3, ReplicationFactor: 2})
	require.NoError(t, err)
	require.Equal(t, int32(3), p)
	require.Equal(t, int16(2), r)

	_, _, err = partitionsAndReplication(TopicConfig{NumPartitions: 1 << 40})
	require.Error(t, err)
	_, _, err = partitionsAndReplication(TopicConfig{ReplicationFactor: 1 << 20})
	require.Error(t, err)
}

func TestBuildReplicaAssignments(t *testing.T) {
	t.Parallel()

	out, err := buildReplicaAssignments([]ReplicaAssignment{
		{Partition: 0, Replicas: []int32{1, 2}},
		{Partition: 1, Replicas: []int32{2, 3}},
	})
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, int32(0), out[0].Partition)
	require.Equal(t, []int32{1, 2}, out[0].Replicas)

	_, err = buildReplicaAssignments([]ReplicaAssignment{{Partition: -1, Replicas: []int32{1}}})
	require.Error(t, err)

	// Duplicate partition.
	_, err = buildReplicaAssignments([]ReplicaAssignment{
		{Partition: 0, Replicas: []int32{1}},
		{Partition: 0, Replicas: []int32{2}},
	})
	require.Error(t, err)

	// Sparse / out-of-range: one entry with partition 10 cannot describe a
	// 1-partition layout (must be contiguous 0..N-1).
	_, err = buildReplicaAssignments([]ReplicaAssignment{{Partition: 10, Replicas: []int32{1}}})
	require.Error(t, err)
}

func TestTopicError(t *testing.T) {
	t.Parallel()
	require.NoError(t, topicError(0, nil)) // 0 = success
	require.Error(t, topicError(3, nil))   // UNKNOWN_TOPIC_OR_PARTITION
	msg := "boom"
	require.ErrorContains(t, topicError(3, &msg), "boom")
}

func TestTopicNames(t *testing.T) {
	t.Parallel()
	strptr := func(s string) *string { return &s }

	// Internal topics and nameless entries are skipped; the rest are returned.
	names, err := topicNames([]kmsg.MetadataResponseTopic{
		{Topic: strptr("orders")},
		{Topic: strptr("__consumer_offsets"), IsInternal: true},
		{Topic: nil}, // skipped: no name
		{Topic: strptr("events")},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"orders", "events"}, names)

	// A per-topic error is surfaced, not silently dropped.
	_, err = topicNames([]kmsg.MetadataResponseTopic{
		{Topic: strptr("orders")},
		{Topic: strptr("broken"), ErrorCode: 3},
	})
	require.Error(t, err)
}

func TestCreateTopicValidation(t *testing.T) {
	t.Parallel()
	conn := newTestConnection(t)

	// Empty topic rejected locally (before any broker round-trip).
	require.Error(t, conn.CreateTopic(TopicConfig{}))

	// Bad replica assignments rejected locally.
	require.Error(t, conn.CreateTopic(TopicConfig{
		Topic:              "t",
		ReplicaAssignments: []ReplicaAssignment{{Partition: -1, Replicas: []int32{1}}},
	}))
	require.Error(t, conn.CreateTopic(TopicConfig{
		Topic: "t",
		ReplicaAssignments: []ReplicaAssignment{
			{Partition: 0, Replicas: []int32{1}},
			{Partition: 0, Replicas: []int32{2}},
		},
	}))
}

func TestDeleteTopicValidation(t *testing.T) {
	t.Parallel()
	conn := newTestConnection(t)
	require.Error(t, conn.DeleteTopic(""))
}

func TestAdminRejectsInitContext(t *testing.T) {
	t.Parallel()
	rt := modulestest.NewRuntime(t) // init context: State is nil
	client, err := kgo.NewClient(kgo.SeedBrokers("b:9092"))
	require.NoError(t, err)
	conn := &Connection{vu: rt.VU, client: client}
	t.Cleanup(conn.Close)

	require.Error(t, conn.CreateTopic(TopicConfig{Topic: "t"}))
	require.Error(t, conn.DeleteTopic("t"))
	_, err = conn.ListTopics()
	require.Error(t, err)
}

func TestAdminAfterCloseErrors(t *testing.T) {
	t.Parallel()
	conn := newTestConnection(t)
	conn.Close() // client = nil

	require.Error(t, conn.CreateTopic(TopicConfig{Topic: "t"}))
	require.Error(t, conn.DeleteTopic("t"))
	_, err := conn.ListTopics()
	require.Error(t, err)
}

func TestConnectionExposesMethods(t *testing.T) {
	t.Parallel()
	rt := modulestest.NewRuntime(t)
	rt.MoveToVUContext(&lib.State{})
	client, err := kgo.NewClient(kgo.SeedBrokers("b:9092"))
	require.NoError(t, err)
	conn := &Connection{vu: rt.VU, client: client}
	t.Cleanup(conn.Close)
	require.NoError(t, rt.VU.Runtime().Set("c", conn))

	v, err := rt.VU.Runtime().RunString(
		`typeof c.createTopic === "function" && typeof c.deleteTopic === "function" && ` +
			`typeof c.listTopics === "function" && typeof c.close === "function"`)
	require.NoError(t, err)
	require.True(t, v.ToBoolean())
}
