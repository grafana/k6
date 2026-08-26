package kafka

import (
	"errors"
)

type errCode uint32

const (
	// non specific.
	kafkaForbiddenInInitContext errCode = 1000
	noContextError              errCode = 1001
	cannotReportStats           errCode = 1002
	fileNotFound                errCode = 1003
	dialerError                 errCode = 1004
	noTLSConfig                 errCode = 1005
	failedTypeCast              errCode = 1006
	unsupportedOperation        errCode = 1007
	writerError                 errCode = 1008
	invalidConfiguration        errCode = 1009
	failedCreateProducer        errCode = 1010
	failedFlushProducer         errCode = 1011
	failedFreezeObject          errCode = 1012

	// serdes errors.
	invalidDataType            errCode = 2000
	failedToEncode             errCode = 2001
	failedToEncodeToBinary     errCode = 2002
	failedToDecodeFromBinary   errCode = 2003
	failedUnmarshalJSON        errCode = 2004
	failedValidateJSON         errCode = 2005
	failedDecodeJSONFromBinary errCode = 2006
	failedUnmarshalSchema      errCode = 2007
	invalidSerdeType           errCode = 2008
	failedDecodeBase64         errCode = 2009

	// consumer.
	failedSetOffset        errCode = 3000
	failedReadMessage      errCode = 3001
	noMoreMessages         errCode = 3002
	partitionAndGroupID    errCode = 3003
	topicAndGroupID        errCode = 3004
	failedParseStartOffset errCode = 3005
	failedCreateConsumer   errCode = 3006
	failedCommitConsumer   errCode = 3007

	// authentication.
	failedCreateDialerWithScram    errCode = 4000
	failedCreateDialerWithSaslSSL  errCode = 4001
	failedLoadX509KeyPair          errCode = 4002
	failedReadCaCertFile           errCode = 4003
	failedAppendCaCertFile         errCode = 4004
	failedCreateDialerWithAwsIam   errCode = 4005
	failedReadJKSFile              errCode = 4006
	failedDecodeJKSFile            errCode = 4007
	failedDecodePrivateKey         errCode = 4008
	failedDecodeServerCa           errCode = 4009
	failedConfigureJKS             errCode = 4010
	failedWriteCertFile            errCode = 4011
	failedWriteKeyFile             errCode = 4012
	failedWriteServerCaFile        errCode = 4013
	failedCreateOAuthTokenProvider errCode = 4014
	failedGetOAuthToken            errCode = 4015

	// schema registry.
	messageTooShort                     errCode = 5000
	schemaNotFound                      errCode = 5001
	schemaCreationFailed                errCode = 5002
	failedConfigureSchemaRegistryClient errCode = 5003
	referenceNotFound                   errCode = 5004
	failedParseReferencedSchema         errCode = 5005
	failedResolveReferences             errCode = 5006
	invalidSchemaID                     errCode = 5007
	protobufSchemaCompileFailed         errCode = 5008
	protobufMissingImport               errCode = 5009
	protobufAmbiguousMessageName        errCode = 5010
	protobufMissingMessageName          errCode = 5011
	protobufInvalidMessageIndexPath     errCode = 5012
	protobufObjectValidationFailed      errCode = 5013
	protobufUnsupportedFormatInput      errCode = 5014

	// topics.
	failedGetController     errCode = 6000
	failedCreateTopic       errCode = 6001
	failedDeleteTopic       errCode = 6002
	failedReadPartitions    errCode = 6003
	failedCreateAdminClient errCode = 6004
	failedGetMetadata       errCode = 6005
)

var (
	// ErrUnsupportedOperation is the error returned when the operation is not supported.
	ErrUnsupportedOperation = NewXk6KafkaError(unsupportedOperation, "Operation not supported", nil)

	// ErrProtobufSchemaCompileFailed is returned when compiling a protobuf schema fails.
	ErrProtobufSchemaCompileFailed = NewXk6KafkaError(
		protobufSchemaCompileFailed,
		"Failed to compile protobuf schema",
		nil,
	)
	// ErrProtobufMissingImport is returned when a protobuf import cannot be resolved.
	ErrProtobufMissingImport = NewXk6KafkaError(
		protobufMissingImport,
		"Missing protobuf import or dependency",
		nil,
	)
	// ErrProtobufAmbiguousMessageName is returned when schema has multiple top-level messages.
	ErrProtobufAmbiguousMessageName = NewXk6KafkaError(
		protobufAmbiguousMessageName,
		"Ambiguous protobuf message name: set schema.messageName",
		nil,
	)
	// ErrProtobufMissingMessageName is returned when the selected message doesn't exist.
	ErrProtobufMissingMessageName = NewXk6KafkaError(
		protobufMissingMessageName,
		"Configured protobuf messageName was not found in schema",
		nil,
	)
	// ErrProtobufInvalidMessageIndexPath is returned when protobuf framing indexes are invalid.
	ErrProtobufInvalidMessageIndexPath = NewXk6KafkaError(
		protobufInvalidMessageIndexPath,
		"Invalid protobuf message-index path",
		nil,
	)
	// ErrProtobufObjectValidationFailed is returned when object-mode payload is invalid.
	ErrProtobufObjectValidationFailed = NewXk6KafkaError(
		protobufObjectValidationFailed,
		"Failed to validate object against protobuf schema",
		nil,
	)
	// ErrProtobufUnsupportedFormatInput is returned when protobufFormat data type is unsupported.
	ErrProtobufUnsupportedFormatInput = NewXk6KafkaError(
		protobufUnsupportedFormatInput,
		"Unsupported input type for protobufFormat",
		nil,
	)

	// ErrForbiddenInInitContext is used when a Kafka producer was used in the init context.
	ErrForbiddenInInitContext = NewXk6KafkaError(
		kafkaForbiddenInInitContext,
		"Producing Kafka messages in the init context is not supported",
		nil)

	// ErrInvalidDataType is used when a data type is not supported.
	ErrInvalidDataType = NewXk6KafkaError(
		invalidDataType,
		"Invalid data type provided for serializer/deserializer",
		nil)

	// ErrInvalidSchema is used when a schema is not supported or is malformed.
	ErrInvalidSchema = NewXk6KafkaError(failedUnmarshalSchema, "Failed to unmarshal schema", nil)

	// ErrFailedTypeCast is used when a type cast failed.
	ErrFailedTypeCast = NewXk6KafkaError(failedTypeCast, "Failed to cast type", nil)

	// ErrUnknownSerdesType is used when a serdes type is not supported.
	ErrUnknownSerdesType = NewXk6KafkaError(invalidSerdeType, "Unknown serdes type", nil)

	ErrPartitionAndGroupID = NewXk6KafkaError(
		partitionAndGroupID, "Partition and groupID cannot be set at the same time", nil)

	ErrTopicAndGroupID = NewXk6KafkaError(
		topicAndGroupID,
		"When you specify groupID, you must set groupTopics instead of topic", nil)

	// ErrNotEnoughArguments is used when a function is called with too few arguments.
	ErrNotEnoughArguments = errors.New("not enough arguments")

	// ErrNoSchemaRegistryClient is used when a schema registry client is not configured correctly.
	ErrNoSchemaRegistryClient = NewXk6KafkaError(
		failedConfigureSchemaRegistryClient,
		"Failed to configure the schema registry client",
		nil)

	// ErrNoJKSConfig is used when a JKS config is not configured correctly.
	ErrNoJKSConfig = NewXk6KafkaError(failedConfigureJKS, "Failed to configure JKS", nil)

	// ErrInvalidPEMData is used when PEM data is invalid.
	ErrInvalidPEMData = errors.New("tls: failed to find any PEM data in certificate input")

	// ErrInvalidDuration is used when duration parsing fails.
	ErrInvalidDuration = errors.New("invalid duration")

	// ErrAvroMissingRequiredField is used when Avro encoding fails due to missing required field.
	ErrAvroMissingRequiredField = errors.New("avro: missing required field key")

	// ErrReferenceNotFound is returned when a schema reference cannot be found.
	ErrReferenceNotFound = errors.New("reference not found in registry")
	// ErrFailedParseReferencedSchema is returned when parsing a referenced schema fails.
	ErrFailedParseReferencedSchema = errors.New("failed to parse referenced schema: returned nil")
	// ErrFailedResolveReferences is returned when resolving schema references fails.
	ErrFailedResolveReferences = errors.New("failed to resolve references")
)

type Xk6KafkaError struct {
	Code          errCode
	Message       string
	OriginalError error
}

// NewXk6KafkaError is the constructor for Xk6KafkaError.
func NewXk6KafkaError(code errCode, msg string, originalErr error) *Xk6KafkaError {
	return &Xk6KafkaError{Code: code, Message: msg, OriginalError: originalErr}
}

// Error implements the `error` interface, so Xk6KafkaError are normal Go errors.
func (e Xk6KafkaError) Error() string {
	if e.OriginalError == nil {
		return e.Message
	}
	return e.Message + ", OriginalError: " + e.OriginalError.Error()
}

// Unwrap implements the `xerrors.Wrapper` interface, so Xk6KafkaError are a bit
// future-proof Go 2 errors.
func (e Xk6KafkaError) Unwrap() error {
	return e.OriginalError
}
