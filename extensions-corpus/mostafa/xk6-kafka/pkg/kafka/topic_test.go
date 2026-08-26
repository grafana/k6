package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicConfigDecodesFromJSShape(t *testing.T) {
	test := getTestModuleInstance(t)

	var topicConfig TopicConfig
	decodeArgumentMap(test.rt, map[string]any{
		"topic":             "test-topic",
		"numPartitions":     3,
		"replicationFactor": 2,
		"replicaAssignments": []map[string]any{
			{
				"partition": 0,
				"replicas":  []int{1, 2},
			},
		},
		"configEntries": []map[string]any{
			{
				"configName":  "cleanup.policy",
				"configValue": "compact",
			},
		},
	}, &topicConfig, "topic config")

	assert.Equal(t, TopicConfig{
		Topic:             "test-topic",
		NumPartitions:     3,
		ReplicationFactor: 2,
		ReplicaAssignments: []ReplicaAssignment{{
			Partition: 0,
			Replicas:  []int32{1, 2},
		}},
		ConfigEntries: []ConfigEntry{{
			ConfigName:  "cleanup.policy",
			ConfigValue: "compact",
		}},
	}, topicConfig)
}

func TestTopicConfigToConfluentSpec(t *testing.T) {
	spec, err := topicConfigToConfluentSpec(TopicConfig{
		Topic:             "test-topic",
		NumPartitions:     1,
		ReplicationFactor: 3,
		ReplicaAssignments: []ReplicaAssignment{
			{
				Partition: 0,
				Replicas:  []int32{1, 2, 3},
			},
			{
				Partition: 2,
				Replicas:  []int32{2, 3, 4},
			},
		},
		ConfigEntries: []ConfigEntry{
			{
				ConfigName:  "cleanup.policy",
				ConfigValue: "compact",
			},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "test-topic", spec.Topic)
	assert.Equal(t, 3, spec.NumPartitions)
	assert.Equal(t, 0, spec.ReplicationFactor)
	assert.Equal(t, [][]int32{
		{1, 2, 3},
		nil,
		{2, 3, 4},
	}, spec.ReplicaAssignment)
	assert.Equal(t, map[string]string{
		"cleanup.policy": "compact",
	}, spec.Config)
}

func TestTopicConfigToConfluentSpecRejectsInvalidReplicaAssignments(t *testing.T) {
	t.Run("negative partition", func(t *testing.T) {
		_, err := topicConfigToConfluentSpec(TopicConfig{
			Topic: "test-topic",
			ReplicaAssignments: []ReplicaAssignment{{
				Partition: -1,
				Replicas:  []int32{1},
			}},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "Invalid topic config, OriginalError: replica assignment partition must not be negative")
	})

	t.Run("duplicate partition", func(t *testing.T) {
		_, err := topicConfigToConfluentSpec(TopicConfig{
			Topic: "test-topic",
			ReplicaAssignments: []ReplicaAssignment{
				{Partition: 0, Replicas: []int32{1}},
				{Partition: 0, Replicas: []int32{2}},
			},
		})
		require.Error(t, err)
		assert.EqualError(t, err, "Invalid topic config, OriginalError: replica assignment partition must be unique")
	})
}

func TestTopicMetadataToJSNil(t *testing.T) {
	assert.Nil(t, topicMetadataToJS(nil))
}
