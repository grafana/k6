package kafka

import (
	"context"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

type ConnectionConfig struct {
	Address string     `json:"address"`
	Brokers []string   `json:"brokers"`
	SASL    SASLConfig `json:"sasl"`
	TLS     TLSConfig  `json:"tls"`
}

func (k *Kafka) adminClientClass(call sobek.ConstructorCall) *sobek.Object {
	return k.compatAdminClientClass(call, false)
}

// connectionClass is a constructor for the Connection object in JS
// that creates a new connection for creating, listing and deleting topics,
// e.g. new Connection(...).
// nolint: funlen
func (k *Kafka) connectionClass(call sobek.ConstructorCall) *sobek.Object {
	return k.compatAdminClientClass(call, true)
}

func (k *Kafka) compatAdminClientClass(
	call sobek.ConstructorCall,
	legacy bool,
) *sobek.Object {
	runtime := k.vu.Runtime()
	var connectionConfig ConnectionConfig
	if len(call.Arguments) == 0 {
		common.Throw(runtime, ErrNotEnoughArguments)
	}

	decodeArgument(runtime, call.Argument(0), &connectionConfig, "connection config")

	adminClient, err := NewAdminClientFromConnectionConfig(&connectionConfig)
	if err != nil {
		common.Throw(runtime, err)
	}

	adminObject := runtime.NewObject()
	if err := adminObject.Set("This", adminClient); err != nil {
		common.Throw(runtime, err)
	}

	err = adminObject.Set("createTopic", func(call sobek.FunctionCall) sobek.Value {
		var topicConfig TopicConfig
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		decodeArgument(runtime, call.Argument(0), &topicConfig, "topic config")

		if err := adminClient.CreateTopic(k.adminContext(), topicConfig); err != nil {
			common.Throw(runtime, err)
		}
		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = adminObject.Set("deleteTopic", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		topic, ok := call.Argument(0).Export().(string)
		if !ok {
			common.Throw(runtime, newInvalidConfigError("topic config", errTopicMustNotBeEmpty))
		}

		if err := adminClient.DeleteTopic(k.adminContext(), topic); err != nil {
			common.Throw(runtime, err)
		}

		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = adminObject.Set("listTopics", func(_ sobek.FunctionCall) sobek.Value {
		topics, err := adminClient.ListTopics(k.adminContext())
		if err != nil {
			common.Throw(runtime, err)
		}
		if legacy {
			topicNames := make([]string, 0, len(topics))
			for _, topic := range topics {
				topicNames = append(topicNames, topic.Topic)
			}
			return runtime.ToValue(topicNames)
		}
		return runtime.ToValue(topicInfosToJS(topics))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = adminObject.Set("getMetadata", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		topic, ok := call.Argument(0).Export().(string)
		if !ok {
			common.Throw(runtime, newInvalidConfigError("topic config", errTopicMustNotBeEmpty))
		}

		metadata, err := adminClient.GetMetadata(k.adminContext(), topic)
		if err != nil {
			common.Throw(runtime, err)
		}
		return runtime.ToValue(topicMetadataToJS(metadata))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = adminObject.Set("close", func(_ sobek.FunctionCall) sobek.Value {
		if err := adminClient.Close(); err != nil {
			common.Throw(runtime, err)
		}

		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	if err := freeze(adminObject); err != nil {
		common.Throw(runtime, err)
	}

	return adminObject
}

func (k *Kafka) adminContext() context.Context {
	return ensureContext(k.vu.Context())
}

func topicInfosToJS(topics []TopicInfo) []map[string]any {
	converted := make([]map[string]any, 0, len(topics))
	for _, topic := range topics {
		converted = append(converted, topicInfoToJS(topic))
	}
	return converted
}

func topicInfoToJS(topic TopicInfo) map[string]any {
	return map[string]any{
		"topic":      topic.Topic,
		"partitions": topic.Partitions,
		"error":      topic.Error,
		// Backward-compatible aliases.
		"Topic":      topic.Topic,
		"Partitions": topic.Partitions,
		"Error":      topic.Error,
	}
}

func partitionInfoToJS(partition PartitionInfo) map[string]any {
	return map[string]any{
		"id":       partition.ID,
		"leader":   partition.Leader,
		"replicas": partition.Replicas,
		"isrs":     partition.Isrs,
		"error":    partition.Error,
		// Backward-compatible aliases.
		"ID":       partition.ID,
		"Leader":   partition.Leader,
		"Replicas": partition.Replicas,
		"Isrs":     partition.Isrs,
		"Error":    partition.Error,
	}
}

func topicMetadataToJS(metadata *TopicMetadata) map[string]any {
	if metadata == nil {
		return nil
	}

	partitions := make([]map[string]any, 0, len(metadata.Partitions))
	for _, partition := range metadata.Partitions {
		partitions = append(partitions, partitionInfoToJS(partition))
	}

	return map[string]any{
		"topic":      metadata.Topic,
		"partitions": partitions,
		"error":      metadata.Error,
		// Backward-compatible aliases.
		"Topic":      metadata.Topic,
		"Partitions": partitions,
		"Error":      metadata.Error,
	}
}
