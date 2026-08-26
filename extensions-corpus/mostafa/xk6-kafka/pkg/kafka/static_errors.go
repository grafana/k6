package kafka

import "errors"

var (
	errAddressMustNotBeEmpty                 = errors.New("address must not be empty")
	errBrokersMustNotBeEmpty                 = errors.New("brokers must not be empty")
	errEmptyTopicResultSet                   = errors.New("empty topic result set")
	errExpectedObject                        = errors.New("expected object")
	errGroupTopicsMustNotBeEmpty             = errors.New("groupTopics must not be empty")
	errNoPositionsReturned                   = errors.New("no positions returned")
	errObjectMustNotBeNil                    = errors.New("object must not be nil")
	errPartitionOutOfRange                   = errors.New("partition is out of int32 range")
	errPositionRequiresSingleConfiguredTopic = errors.New("position requires a single configured topic")
	errReplicaAssignmentPartitionNegative    = errors.New("replica assignment partition must not be negative")
	errReplicaAssignmentPartitionUnique      = errors.New("replica assignment partition must be unique")
	errRequiredAcksInvalid                   = errors.New("requiredAcks must be one of -1, 0, or 1")
	errSchemaMustNotBeEmpty                  = errors.New("schema must not be empty")
	errSchemaTypeMustNotBeEmpty              = errors.New("schemaType must not be empty")
	errSeekRequiresSingleConfiguredTopic     = errors.New("seek requires a single configured topic")
	errStartOffsetInvalid                    = errors.New(
		"startOffset must be FIRST_OFFSET, LAST_OFFSET, or a numeric offset",
	)
	errSubjectMustNotBeEmpty = errors.New("subject must not be empty")
	errTopicMetadataNotFound = errors.New("topic metadata not found")
	errTopicMustNotBeEmpty   = errors.New("topic must not be empty")
	errUnknownBalancer       = errors.New("unknown balancer")
	errURLMustNotBeEmpty     = errors.New("url must not be empty")
)
