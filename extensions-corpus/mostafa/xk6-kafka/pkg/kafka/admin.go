package kafka

import (
	"context"
	"sort"
	"sync"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type TopicConfig struct {
	Topic              string              `json:"topic"`
	NumPartitions      int                 `json:"numPartitions"`
	ReplicationFactor  int                 `json:"replicationFactor"`
	ReplicaAssignments []ReplicaAssignment `json:"replicaAssignments"`
	ConfigEntries      []ConfigEntry       `json:"configEntries"`
}

type ReplicaAssignment struct {
	Partition int     `json:"partition"`
	Replicas  []int32 `json:"replicas"`
}

type ConfigEntry struct {
	ConfigName  string `json:"configName"`
	ConfigValue string `json:"configValue"`
}

type TopicInfo struct {
	Topic      string
	Partitions int
	Error      error
}

type PartitionInfo struct {
	ID       int32
	Leader   int32
	Replicas []int32
	Isrs     []int32
	Error    error
}

type TopicMetadata struct {
	Topic      string
	Partitions []PartitionInfo
	Error      error
}

type AdminClient struct {
	client    *ckafka.AdminClient
	pClient   *ckafka.Producer
	config    ckafka.ConfigMap
	doneChan  chan struct{}
	closeOnce sync.Once
}

func NewAdminClientFromConnectionConfig(connectionConfig *ConnectionConfig) (*AdminClient, error) {
	config, err := connectionConfigToConfluentConfigMap(connectionConfig)
	if err != nil {
		return nil, err
	}

	saslContext, err := NewSaslContext(connectionConfig.SASL, connectionConfig.Brokers, SASLContextOpts{})
	if err != nil {
		return nil, err
	}

	// The admin client does not have an event channel to detect requests for OAuth tokens.
	// Creating the admin client through the producer will provide an event channel for
	// OAuth events.
	pClient, err := ckafka.NewProducer(&config)
	if err != nil {
		return nil, NewXk6KafkaError(failedCreateAdminClient, "Failed to create admin client.", err)
	}

	client, err := ckafka.NewAdminClientFromProducer(pClient)
	if err != nil {
		pClient.Close()
		return nil, NewXk6KafkaError(failedCreateAdminClient, "Failed to create admin client.", err)
	}

	doneChan := make(chan struct{})
	go handleProducerClientEvents(saslContext, pClient, pClient.Events(), doneChan)

	return &AdminClient{
		client:   client,
		pClient:  pClient,
		config:   cloneConfluentConfigMap(config),
		doneChan: doneChan,
	}, nil
}

func (a *AdminClient) CreateTopic(ctx context.Context, config TopicConfig) error {
	if a == nil || a.client == nil {
		return newMissingConfigError("admin client")
	}
	ctx = ensureContext(ctx)
	spec, err := topicConfigToConfluentSpec(config)
	if err != nil {
		return err
	}

	results, err := a.client.CreateTopics(ctx, []ckafka.TopicSpecification{spec})
	if err != nil {
		return NewXk6KafkaError(failedCreateTopic, "Failed to create topic.", err)
	}

	return adminTopicResultError(results, failedCreateTopic)
}

func (a *AdminClient) DeleteTopic(ctx context.Context, name string) error {
	if a == nil || a.client == nil {
		return newMissingConfigError("admin client")
	}
	ctx = ensureContext(ctx)
	if name == "" {
		return newInvalidConfigError("topic config", errTopicMustNotBeEmpty)
	}

	results, err := a.client.DeleteTopics(ctx, []string{name})
	if err != nil {
		return NewXk6KafkaError(failedDeleteTopic, "Failed to delete topic.", err)
	}

	return adminTopicResultError(results, failedDeleteTopic)
}

func (a *AdminClient) ListTopics(ctx context.Context) ([]TopicInfo, error) {
	if a == nil || a.client == nil {
		return nil, newMissingConfigError("admin client")
	}
	ctx = ensureContext(ctx)

	metadata, err := a.client.GetMetadata(nil, true, confluentMetadataTimeoutMs(ctx))
	if err != nil {
		return nil, NewXk6KafkaError(failedGetMetadata, "Failed to list topic metadata.", err)
	}

	topics := make([]TopicInfo, 0, len(metadata.Topics))
	for _, topicMetadata := range metadata.Topics {
		topics = append(topics, TopicInfo{
			Topic:      topicMetadata.Topic,
			Partitions: len(topicMetadata.Partitions),
			Error:      normalizeConfluentError(topicMetadata.Error),
		})
	}

	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Topic < topics[j].Topic
	})

	return topics, nil
}

func (a *AdminClient) GetMetadata(ctx context.Context, topic string) (*TopicMetadata, error) {
	if a == nil || a.client == nil {
		return nil, newMissingConfigError("admin client")
	}
	ctx = ensureContext(ctx)
	if topic == "" {
		return nil, newInvalidConfigError("topic config", errTopicMustNotBeEmpty)
	}

	metadata, err := a.client.GetMetadata(&topic, false, confluentMetadataTimeoutMs(ctx))
	if err != nil {
		return nil, NewXk6KafkaError(failedGetMetadata, "Failed to get topic metadata.", err)
	}

	topicMetadata, ok := metadata.Topics[topic]
	if !ok {
		return nil, NewXk6KafkaError(
			failedGetMetadata,
			"Topic metadata was not returned.",
			errTopicMetadataNotFound,
		)
	}

	converted := &TopicMetadata{
		Topic:      topicMetadata.Topic,
		Partitions: make([]PartitionInfo, 0, len(topicMetadata.Partitions)),
		Error:      normalizeConfluentError(topicMetadata.Error),
	}

	for _, partition := range topicMetadata.Partitions {
		converted.Partitions = append(converted.Partitions, PartitionInfo{
			ID:       partition.ID,
			Leader:   partition.Leader,
			Replicas: append([]int32(nil), partition.Replicas...),
			Isrs:     append([]int32(nil), partition.Isrs...),
			Error:    normalizeConfluentError(partition.Error),
		})
	}

	return converted, nil
}

func (a *AdminClient) Close() error {
	if a == nil {
		return nil
	}

	a.closeOnce.Do(func() {
		if a.doneChan != nil {
			close(a.doneChan)
			a.doneChan = nil
		}

		client := a.client
		a.client = nil
		if client != nil {
			client.Close()
		}

		pClient := a.pClient
		a.pClient = nil
		if pClient != nil {
			pClient.Close()
		}
	})

	return nil
}

func adminTopicResultError(results []ckafka.TopicResult, code errCode) error {
	if len(results) == 0 {
		return NewXk6KafkaError(code, "Admin operation returned no topic results.", errEmptyTopicResultSet)
	}

	result := results[0]
	if result.Error.Code() == ckafka.ErrNoError {
		return nil
	}

	return NewXk6KafkaError(code, "Admin topic operation failed.", result.Error)
}

func normalizeConfluentError(err ckafka.Error) error {
	if err.Code() == ckafka.ErrNoError {
		return nil
	}

	return err
}

func topicConfigToConfluentSpec(config TopicConfig) (ckafka.TopicSpecification, error) {
	if config.Topic == "" {
		return ckafka.TopicSpecification{}, newInvalidConfigError("topic config", errTopicMustNotBeEmpty)
	}
	if config.NumPartitions <= 0 {
		config.NumPartitions = 1
	}
	if config.ReplicationFactor <= 0 {
		config.ReplicationFactor = 1
	}

	configMap := make(map[string]string, len(config.ConfigEntries))
	for _, entry := range config.ConfigEntries {
		configMap[entry.ConfigName] = entry.ConfigValue
	}

	var replicaAssignment [][]int32
	if len(config.ReplicaAssignments) > 0 {
		maxPartition := -1
		seenPartitions := make(map[int]struct{}, len(config.ReplicaAssignments))
		for _, assignment := range config.ReplicaAssignments {
			if assignment.Partition < 0 {
				return ckafka.TopicSpecification{}, newInvalidConfigError(
					"topic config",
					errReplicaAssignmentPartitionNegative,
				)
			}
			if _, exists := seenPartitions[assignment.Partition]; exists {
				return ckafka.TopicSpecification{}, newInvalidConfigError(
					"topic config",
					errReplicaAssignmentPartitionUnique,
				)
			}
			seenPartitions[assignment.Partition] = struct{}{}
			if assignment.Partition > maxPartition {
				maxPartition = assignment.Partition
			}
		}

		replicaAssignment = make([][]int32, maxPartition+1)
		for _, assignment := range config.ReplicaAssignments {
			replicaAssignment[assignment.Partition] = append([]int32(nil), assignment.Replicas...)
		}
		config.ReplicationFactor = 0
		if config.NumPartitions < len(replicaAssignment) {
			config.NumPartitions = len(replicaAssignment)
		}
	}

	return ckafka.TopicSpecification{
		Topic:             config.Topic,
		NumPartitions:     config.NumPartitions,
		ReplicationFactor: config.ReplicationFactor,
		ReplicaAssignment: replicaAssignment,
		Config:            configMap,
	}, nil
}
