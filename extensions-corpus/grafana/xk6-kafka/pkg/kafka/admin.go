package kafka

import (
	"errors"
	"fmt"
	"math"

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// defaultPartitions and defaultReplicationFactor are used when numPartitions /
// replicationFactor are unset (and no replicaAssignments are given).
const (
	defaultPartitions        = 1
	defaultReplicationFactor = 1
)

// TopicConfig configures a topic to create (see index.d.ts TopicConfig).
type TopicConfig struct {
	Topic              string              `js:"topic"`
	NumPartitions      int                 `js:"numPartitions"`
	ReplicationFactor  int                 `js:"replicationFactor"`
	ReplicaAssignments []ReplicaAssignment `js:"replicaAssignments"`
	ConfigEntries      []ConfigEntry       `js:"configEntries"`
}

// ReplicaAssignment maps a partition to the broker IDs hosting its replicas.
type ReplicaAssignment struct {
	Partition int32   `js:"partition"`
	Replicas  []int32 `js:"replicas"`
}

// ConfigEntry is a single topic-level configuration entry.
type ConfigEntry struct {
	ConfigName  string `js:"configName"`
	ConfigValue string `js:"configValue"`
}

// kmsgReplicaAssignment aliases the verbose protocol type for readability.
type kmsgReplicaAssignment = kmsg.CreateTopicsRequestTopicReplicaAssignment

// CreateTopic creates a topic from the config. It validates input locally before
// contacting the broker, then issues a CreateTopics request and surfaces any
// per-topic error.
func (c *Connection) CreateTopic(cfg TopicConfig) error {
	if err := c.requireVU("createTopic"); err != nil {
		return err
	}
	if cfg.Topic == "" {
		return errors.New("createTopic: topic must not be empty")
	}

	rt := kmsg.NewCreateTopicsRequestTopic()
	rt.Topic = cfg.Topic
	if len(cfg.ReplicaAssignments) > 0 {
		// The assignment list fully determines the layout; the protocol requires
		// NumPartitions and ReplicationFactor to be -1 in this mode.
		rt.NumPartitions = -1
		rt.ReplicationFactor = -1
		assignments, err := buildReplicaAssignments(cfg.ReplicaAssignments)
		if err != nil {
			return err
		}
		rt.ReplicaAssignment = assignments
	} else {
		partitions, replication, err := partitionsAndReplication(cfg)
		if err != nil {
			return err
		}
		rt.NumPartitions = partitions
		rt.ReplicationFactor = replication
	}
	for _, e := range cfg.ConfigEntries {
		rc := kmsg.NewCreateTopicsRequestTopicConfig()
		rc.Name = e.ConfigName
		value := e.ConfigValue
		rc.Value = &value
		rt.Configs = append(rt.Configs, rc)
	}

	req := kmsg.NewPtrCreateTopicsRequest()
	req.Topics = append(req.Topics, rt)

	resp, err := req.RequestWith(c.vu.Context(), c.client)
	if err != nil {
		return fmt.Errorf("createTopic %q: %w", cfg.Topic, err)
	}
	for _, t := range resp.Topics {
		if topicErr := topicError(t.ErrorCode, t.ErrorMessage); topicErr != nil {
			return fmt.Errorf("createTopic %q: %w", t.Topic, topicErr)
		}
	}
	return nil
}

// DeleteTopic requests deletion of the named topic and returns once the broker
// accepts the request. Kafka removes the topic asynchronously, so it is not
// guaranteed to be gone from metadata immediately afterwards.
func (c *Connection) DeleteTopic(topic string) error {
	if err := c.requireVU("deleteTopic"); err != nil {
		return err
	}
	if topic == "" {
		return errors.New("deleteTopic: topic must not be empty")
	}

	req := kmsg.NewPtrDeleteTopicsRequest()
	// Set both the legacy TopicNames (v0-v5) and the v6+ Topics so the negotiated
	// request version carries the name on old and new brokers alike.
	req.TopicNames = []string{topic}
	dt := kmsg.NewDeleteTopicsRequestTopic()
	name := topic
	dt.Topic = &name
	req.Topics = append(req.Topics, dt)

	resp, err := req.RequestWith(c.vu.Context(), c.client)
	if err != nil {
		return fmt.Errorf("deleteTopic %q: %w", topic, err)
	}
	for _, t := range resp.Topics {
		if topicErr := topicError(t.ErrorCode, t.ErrorMessage); topicErr != nil {
			return fmt.Errorf("deleteTopic %q: %w", topic, topicErr)
		}
	}
	return nil
}

// ListTopics returns the names of the cluster's non-internal topics.
func (c *Connection) ListTopics() ([]string, error) {
	if err := c.requireVU("listTopics"); err != nil {
		return nil, err
	}

	// A metadata request with no topics returns metadata for every topic.
	req := kmsg.NewPtrMetadataRequest()
	resp, err := req.RequestWith(c.vu.Context(), c.client)
	if err != nil {
		return nil, fmt.Errorf("listTopics: %w", err)
	}
	// Surface a top-level metadata error (e.g. auth, rebootstrap on Kafka 4.0+)
	// rather than returning a silently truncated list.
	if respErr := topicError(resp.ErrorCode, nil); respErr != nil {
		return nil, fmt.Errorf("listTopics: %w", respErr)
	}

	return topicNames(resp.Topics)
}

// topicNames extracts the names of non-internal topics from a metadata response.
// A per-topic error code is surfaced (not hidden by dropping the topic); entries
// with no name are skipped, and internal topics are excluded.
func topicNames(topics []kmsg.MetadataResponseTopic) ([]string, error) {
	names := make([]string, 0, len(topics))
	for _, t := range topics {
		name := ""
		if t.Topic != nil {
			name = *t.Topic
		}
		if topicErr := topicError(t.ErrorCode, nil); topicErr != nil {
			return nil, fmt.Errorf("listTopics: topic %q: %w", name, topicErr)
		}
		if t.IsInternal || t.Topic == nil {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// buildReplicaAssignments validates and maps manual replica assignments. The
// entries must describe a dense partition layout: one entry per partition with
// IDs covering exactly 0..N-1 (unique, non-negative, none beyond the count), so
// the assignment list alone determines the partition count.
func buildReplicaAssignments(assignments []ReplicaAssignment) ([]kmsgReplicaAssignment, error) {
	if len(assignments) > math.MaxInt32 {
		return nil, fmt.Errorf("createTopic: too many replica assignments (%d)", len(assignments))
	}
	count := int32(len(assignments)) // #nosec G115 -- bounded by the check above
	seen := make(map[int32]struct{}, len(assignments))
	out := make([]kmsgReplicaAssignment, count)
	for _, a := range assignments {
		if a.Partition < 0 || a.Partition >= count {
			return nil, fmt.Errorf(
				"createTopic: replica assignment partition %d out of range; partitions must cover 0..%d",
				a.Partition, count-1)
		}
		if _, dup := seen[a.Partition]; dup {
			return nil, fmt.Errorf("createTopic: duplicate replica assignment for partition %d", a.Partition)
		}
		seen[a.Partition] = struct{}{}

		ra := kmsg.NewCreateTopicsRequestTopicReplicaAssignment()
		ra.Partition = a.Partition
		ra.Replicas = a.Replicas
		out[a.Partition] = ra
	}
	return out, nil
}

// partitionsAndReplication resolves the partition count and replication factor,
// defaulting each to 1 when unset, with range guards for the protocol's int32 /
// int16 fields.
func partitionsAndReplication(cfg TopicConfig) (int32, int16, error) {
	partitions := cfg.NumPartitions
	if partitions <= 0 {
		partitions = defaultPartitions
	}
	if partitions > math.MaxInt32 {
		return 0, 0, fmt.Errorf("createTopic: numPartitions %d is too large", partitions)
	}

	replication := cfg.ReplicationFactor
	if replication <= 0 {
		replication = defaultReplicationFactor
	}
	if replication > math.MaxInt16 {
		return 0, 0, fmt.Errorf("createTopic: replicationFactor %d is too large", replication)
	}

	return int32(partitions), int16(replication), nil
}

// topicError converts a Kafka per-topic error code (and optional broker message)
// to an error, or nil when the code is success.
func topicError(code int16, message *string) error {
	err := kerr.ErrorForCode(code)
	if err == nil {
		return nil
	}
	if message != nil && *message != "" {
		return fmt.Errorf("%w: %s", err, *message)
	}
	return err
}
