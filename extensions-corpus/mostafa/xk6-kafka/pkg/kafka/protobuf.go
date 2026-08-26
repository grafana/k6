package kafka

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"go.k6.io/k6/js/common"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type ProtobufSerde struct {
	Serdes
}

type protobufRuntime struct {
	fileDesc    protoreflect.FileDescriptor
	messageDesc protoreflect.MessageDescriptor
	indexes     []int
}

const (
	protobufFormatObject = "object"
	protobufFormatBytes  = "bytes"
)

func normalizeProtobufFormat(format string) string {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		return protobufFormatObject
	}
	return normalized
}

func parseProtobufBytesInput(data any) ([]byte, *Xk6KafkaError) {
	switch value := data.(type) {
	case []byte:
		return value, nil
	case []any:
		bytesData := make([]byte, len(value))
		for i, element := range value {
			converted, ok := element.(float64)
			if !ok {
				return nil, ErrProtobufUnsupportedFormatInput
			}
			bytesData[i] = byte(converted)
		}
		return bytesData, nil
	case string:
		if !isBase64Encoded(value) {
			return nil, ErrProtobufUnsupportedFormatInput
		}
		decoded, err := base64ToBytes(value)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	default:
		return nil, ErrProtobufUnsupportedFormatInput
	}
}

func parseProtobufMessageIndexes(payload []byte) (int, []int, *Xk6KafkaError) {
	arrayLen, bytesRead := binary.Varint(payload)
	if bytesRead <= 0 || arrayLen < 0 {
		return 0, nil, ErrProtobufInvalidMessageIndexPath
	}

	if arrayLen == 0 {
		return bytesRead, []int{0}, nil
	}

	indexes := make([]int, arrayLen)
	for i := range int(arrayLen) {
		index, read := binary.Varint(payload[bytesRead:])
		if read <= 0 {
			return 0, nil, ErrProtobufInvalidMessageIndexPath
		}
		bytesRead += read
		indexes[i] = int(index)
	}

	return bytesRead, indexes, nil
}

func encodeProtobufMessageIndexes(indexes []int) []byte {
	if len(indexes) == 1 && indexes[0] == 0 {
		return []byte{0}
	}

	buffer := make([]byte, (1+len(indexes))*binary.MaxVarintLen64)
	length := binary.PutVarint(buffer, int64(len(indexes)))
	for _, index := range indexes {
		length += binary.PutVarint(buffer[length:], int64(index))
	}
	return buffer[:length]
}

func (k *Kafka) encodeProtobufWireFormat(data []byte, schemaID int, messageIndexes []int) []byte {
	schemaIDBytes := make([]byte, MagicPrefixSize-1)
	if schemaID < 0 || schemaID > int(^uint32(0)) {
		err := NewXk6KafkaError(
			invalidSchemaID,
			fmt.Sprintf("Invalid schema id %d: must be within uint32 range", schemaID),
			nil,
		)
		common.Throw(k.vu.Runtime(), err)
		return nil
	}
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(schemaID))

	payload := make([]byte, 0, 1+len(schemaIDBytes)+len(messageIndexes)+len(data))
	payload = append(payload, 0)
	payload = append(payload, schemaIDBytes...)
	payload = append(payload, encodeProtobufMessageIndexes(messageIndexes)...)
	payload = append(payload, data...)
	return payload
}

func (k *Kafka) decodeProtobufWireFormat(message []byte) (int, []int, []byte, *Xk6KafkaError) {
	if len(message) < MagicPrefixSize {
		return 0, nil, nil, NewXk6KafkaError(
			messageTooShort,
			"Invalid message: message too short to contain schema id.",
			nil,
		)
	}
	if message[0] != 0 {
		return 0, nil, nil, NewXk6KafkaError(
			messageTooShort,
			"Invalid message: invalid start byte.",
			nil,
		)
	}

	schemaID := int(binary.BigEndian.Uint32(message[1:5]))
	bytesRead, indexes, err := parseProtobufMessageIndexes(message[5:])
	if err != nil {
		return 0, nil, nil, err
	}

	return schemaID, indexes, message[5+bytesRead:], nil
}

func toMessageIndexes(descriptor protoreflect.Descriptor, count int) []int {
	index := descriptor.Index()
	switch parent := descriptor.Parent().(type) {
	case protoreflect.FileDescriptor:
		msgIndexes := make([]int, count+1)
		msgIndexes[0] = index
		return msgIndexes[:1]
	default:
		msgIndexes := toMessageIndexes(parent, count+1)
		return append(msgIndexes, index)
	}
}

func toMessageIndexArray(messageDesc protoreflect.MessageDescriptor) []int {
	if messageDesc.Index() == 0 {
		if _, ok := messageDesc.Parent().(protoreflect.FileDescriptor); ok {
			return []int{0}
		}
	}
	return toMessageIndexes(messageDesc, 0)
}

func toMessageDescriptor(
	descriptor protoreflect.Descriptor,
	indexes []int,
) (protoreflect.MessageDescriptor, *Xk6KafkaError) {
	if len(indexes) == 0 {
		return nil, ErrProtobufInvalidMessageIndexPath
	}

	index := indexes[0]
	switch value := descriptor.(type) {
	case protoreflect.FileDescriptor:
		messages := value.Messages()
		if index < 0 || index >= messages.Len() {
			return nil, ErrProtobufInvalidMessageIndexPath
		}
		if len(indexes) == 1 {
			return messages.Get(index), nil
		}
		return toMessageDescriptor(messages.Get(index), indexes[1:])
	case protoreflect.MessageDescriptor:
		messages := value.Messages()
		if index < 0 || index >= messages.Len() {
			return nil, ErrProtobufInvalidMessageIndexPath
		}
		if len(indexes) == 1 {
			return messages.Get(index), nil
		}
		return toMessageDescriptor(messages.Get(index), indexes[1:])
	default:
		return nil, ErrProtobufInvalidMessageIndexPath
	}
}

func findMessageByFullName(
	messages protoreflect.MessageDescriptors,
	fullName protoreflect.FullName,
) protoreflect.MessageDescriptor {
	for i := range messages.Len() {
		message := messages.Get(i)
		if message.FullName() == fullName {
			return message
		}
		nested := findMessageByFullName(message.Messages(), fullName)
		if nested != nil {
			return nested
		}
	}
	return nil
}

func resolveTargetProtobufMessage(
	fileDesc protoreflect.FileDescriptor,
	messageName string,
) (protoreflect.MessageDescriptor, *Xk6KafkaError) {
	if fileDesc == nil {
		return nil, ErrProtobufSchemaCompileFailed
	}

	trimmed := strings.TrimSpace(messageName)
	if trimmed != "" {
		fullName := protoreflect.FullName(trimmed)
		if message := findMessageByFullName(fileDesc.Messages(), fullName); message != nil {
			return message, nil
		}
		return nil, ErrProtobufMissingMessageName
	}

	topLevel := fileDesc.Messages()
	if topLevel.Len() != 1 {
		return nil, ErrProtobufAmbiguousMessageName
	}

	return topLevel.Get(0), nil
}

func resolveReferenceByName(schema *Schema, name string) (*Schema, error) {
	if schema == nil || schema.resolver == nil {
		return nil, ErrReferenceNotFound
	}

	if resolved, err := schema.resolver(name); err == nil && resolved != nil {
		return resolved, nil
	}

	for _, ref := range schema.References {
		if ref.Name == name || ref.Subject == name {
			if resolved, err := schema.resolver(ref.Subject); err == nil && resolved != nil {
				return resolved, nil
			}
		}
	}

	return nil, ErrReferenceNotFound
}

func collectProtobufReferenceSchemas(
	schema *Schema,
	out map[string]string,
	visited map[string]bool,
) *Xk6KafkaError {
	for _, ref := range schema.References {
		if visited[ref.Name] {
			continue
		}

		resolved, err := resolveReferenceByName(schema, ref.Name)
		if err != nil {
			return NewXk6KafkaError(failedResolveReferences, "Failed to resolve protobuf references", err)
		}

		visited[ref.Name] = true
		out[ref.Name] = resolved.Schema
		if collectErr := collectProtobufReferenceSchemas(resolved, out, visited); collectErr != nil {
			return collectErr
		}
	}

	return nil
}

func buildProtobufDependencyMap(schema *Schema) (map[string]string, *Xk6KafkaError) {
	dependencies := make(map[string]string)
	maps.Copy(dependencies, schema.Dependencies)

	if len(schema.References) == 0 {
		return dependencies, nil
	}

	if schema.resolver == nil {
		return nil, NewXk6KafkaError(failedResolveReferences, "Failed to resolve protobuf references", ErrReferenceNotFound)
	}

	if err := collectProtobufReferenceSchemas(schema, dependencies, map[string]bool{}); err != nil {
		return nil, err
	}

	return dependencies, nil
}

// protobufFileDescCache memoises protocompile.Compile results across the
// process. The JS-facing serialize/deserialize handlers go through
// decodeArgument, which json.Marshal/Unmarshal-round-trips the argument
// and so produces a fresh *Schema on every call; a cache keyed on *Schema
// pointer identity therefore cannot hit. Keys here are a SHA-256 hash of
// the schema source plus sorted dependency map, so two distinct *Schema
// instances with identical content share a compiled FileDescriptor.
//
// FileDescriptors are read-only once compiled and safe to share across
// goroutines. The cache is unbounded by design — load tests typically use
// a small fixed set of schemas, so growth is naturally constrained.
var protobufFileDescCache sync.Map

type protobufCachedDescriptor struct {
	once     sync.Once
	fileDesc protoreflect.FileDescriptor
	err      *Xk6KafkaError
}

func parseProtobufFileDescriptor(schema *Schema) (protoreflect.FileDescriptor, *Xk6KafkaError) {
	if schema == nil || strings.TrimSpace(schema.Schema) == "" {
		return nil, ErrProtobufSchemaCompileFailed
	}

	dependencies, err := buildProtobufDependencyMap(schema)
	if err != nil {
		return nil, err
	}

	key := protobufCacheKey(schema.Schema, dependencies)
	entry, _ := protobufFileDescCache.LoadOrStore(key, &protobufCachedDescriptor{})
	cached := entry.(*protobufCachedDescriptor)
	cached.once.Do(func() {
		cached.fileDesc, cached.err = compileProtobufFileDescriptor(schema.Schema, dependencies)
	})
	return cached.fileDesc, cached.err
}

// protobufCacheKey returns a SHA-256 digest of the schema source and the
// canonical (sorted) dependency map. NUL separators after every field
// prevent boundary ambiguities between adjacent keys/values.
func protobufCacheKey(schemaSource string, dependencies map[string]string) string {
	h := sha256.New()
	h.Write([]byte(schemaSource))
	h.Write([]byte{0})

	keys := make([]string, 0, len(dependencies))
	for k := range dependencies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(dependencies[k]))
		h.Write([]byte{0})
	}
	return string(h.Sum(nil))
}

func compileProtobufFileDescriptor(
	schemaSource string,
	dependencies map[string]string,
) (protoreflect.FileDescriptor, *Xk6KafkaError) {
	sources := map[string]string{
		".": schemaSource,
	}
	maps.Copy(sources, dependencies)

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(sources),
		}),
	}

	files, parseErr := compiler.Compile(context.Background(), ".")
	if parseErr != nil {
		if strings.Contains(parseErr.Error(), "file does not exist") ||
			strings.Contains(parseErr.Error(), "not found") {
			return nil, NewXk6KafkaError(
				protobufMissingImport,
				"Missing protobuf import or dependency",
				parseErr,
			)
		}
		return nil, NewXk6KafkaError(failedToEncode, "Failed to compile protobuf schema", parseErr)
	}
	if len(files) != 1 {
		return nil, ErrProtobufSchemaCompileFailed
	}

	return files[0], nil
}

func buildProtobufRuntime(schema *Schema) (*protobufRuntime, *Xk6KafkaError) {
	fileDesc, err := parseProtobufFileDescriptor(schema)
	if err != nil {
		return nil, err
	}

	messageDesc, err := resolveTargetProtobufMessage(fileDesc, schema.MessageName)
	if err != nil {
		return nil, err
	}

	return &protobufRuntime{
		fileDesc:    fileDesc,
		messageDesc: messageDesc,
		indexes:     toMessageIndexArray(messageDesc),
	}, nil
}

func (s *ProtobufSerde) Serialize(data any, schema *Schema) ([]byte, *Xk6KafkaError) {
	runtime, err := buildProtobufRuntime(schema)
	if err != nil {
		return nil, err
	}

	message := dynamicpb.NewMessage(runtime.messageDesc)

	jsonData, err := toJSONBytes(data)
	if err != nil {
		return nil, ErrProtobufObjectValidationFailed
	}

	unmarshalOptions := protojson.UnmarshalOptions{DiscardUnknown: false}
	if unmarshalErr := unmarshalOptions.Unmarshal(jsonData, message); unmarshalErr != nil {
		return nil, NewXk6KafkaError(failedToEncode, "Failed to encode protobuf object data", unmarshalErr)
	}

	encoded, marshalErr := proto.Marshal(message)
	if marshalErr != nil {
		return nil, NewXk6KafkaError(failedToEncodeToBinary, "Failed to encode protobuf data into binary", marshalErr)
	}

	return encoded, nil
}

func (s *ProtobufSerde) Deserialize(data []byte, schema *Schema) (any, *Xk6KafkaError) {
	runtime, err := buildProtobufRuntime(schema)
	if err != nil {
		return nil, err
	}

	message := dynamicpb.NewMessage(runtime.messageDesc)
	if unmarshalErr := proto.Unmarshal(data, message); unmarshalErr != nil {
		return nil, NewXk6KafkaError(failedToDecodeFromBinary, "Failed to decode protobuf data", unmarshalErr)
	}

	jsonData, marshalErr := protojson.MarshalOptions{
		UseProtoNames: false,
	}.Marshal(message)
	if marshalErr != nil {
		return nil, NewXk6KafkaError(failedToDecodeFromBinary, "Failed to convert protobuf data to JSON", marshalErr)
	}

	decoded, mapErr := toMap(jsonData)
	if mapErr != nil {
		return nil, mapErr
	}
	return decoded, nil
}

func (k *Kafka) serializeProtobuf(container *Container) []byte {
	format := normalizeProtobufFormat(container.ProtobufFormat)
	if format != protobufFormatObject && format != protobufFormatBytes {
		common.Throw(k.vu.Runtime(), ErrProtobufUnsupportedFormatInput)
		return nil
	}

	runtime, err := buildProtobufRuntime(container.Schema)
	if err != nil {
		common.Throw(k.vu.Runtime(), err)
		return nil
	}

	var payload []byte
	switch format {
	case protobufFormatObject:
		serde, serdesErr := GetSerdes(Protobuf)
		if serdesErr != nil {
			common.Throw(k.vu.Runtime(), serdesErr)
			return nil
		}
		encoded, encodeErr := serde.Serialize(container.Data, container.Schema)
		if encodeErr != nil {
			common.Throw(k.vu.Runtime(), encodeErr)
			return nil
		}
		payload = encoded
	case protobufFormatBytes:
		rawBytes, bytesErr := parseProtobufBytesInput(container.Data)
		if bytesErr != nil {
			common.Throw(k.vu.Runtime(), bytesErr)
			return nil
		}

		validationMessage := dynamicpb.NewMessage(runtime.messageDesc)
		if unmarshalErr := proto.Unmarshal(rawBytes, validationMessage); unmarshalErr != nil {
			common.Throw(k.vu.Runtime(), NewXk6KafkaError(
				failedToEncodeToBinary,
				"Failed to validate protobuf binary input",
				unmarshalErr,
			))
			return nil
		}
		payload = rawBytes
	}

	schemaID := container.Schema.ID
	if schemaID == 0 {
		schemaID = 0
	}

	return k.encodeProtobufWireFormat(payload, schemaID, runtime.indexes)
}

func (k *Kafka) deserializeProtobuf(container *Container) any {
	format := normalizeProtobufFormat(container.ProtobufFormat)
	if format != protobufFormatObject && format != protobufFormatBytes {
		common.Throw(k.vu.Runtime(), ErrProtobufUnsupportedFormatInput)
		return nil
	}

	var data []byte
	switch payload := container.Data.(type) {
	case []byte:
		data = payload
	case string:
		if isBase64Encoded(payload) {
			decoded, err := base64ToBytes(payload)
			if err != nil {
				common.Throw(k.vu.Runtime(), err)
				return nil
			}
			data = decoded
		} else {
			data = []byte(payload)
		}
	default:
		common.Throw(k.vu.Runtime(), ErrProtobufUnsupportedFormatInput)
		return nil
	}

	_, indexes, payload, err := k.decodeProtobufWireFormat(data)
	if err != nil {
		common.Throw(k.vu.Runtime(), err)
		return nil
	}

	if format == protobufFormatBytes {
		return payload
	}

	fileDesc, parseErr := parseProtobufFileDescriptor(container.Schema)
	if parseErr != nil {
		common.Throw(k.vu.Runtime(), parseErr)
		return nil
	}

	messageDesc, descErr := toMessageDescriptor(fileDesc, indexes)
	if descErr != nil {
		common.Throw(k.vu.Runtime(), descErr)
		return nil
	}

	message := dynamicpb.NewMessage(messageDesc)
	if unmarshalErr := proto.Unmarshal(payload, message); unmarshalErr != nil {
		common.Throw(k.vu.Runtime(), NewXk6KafkaError(
			failedToDecodeFromBinary,
			"Failed to decode protobuf payload",
			unmarshalErr,
		))
		return nil
	}

	jsonData, marshalErr := protojson.MarshalOptions{
		UseProtoNames: false,
	}.Marshal(message)
	if marshalErr != nil {
		common.Throw(k.vu.Runtime(), NewXk6KafkaError(
			failedToDecodeFromBinary,
			"Failed to convert protobuf payload to JSON object",
			marshalErr,
		))
		return nil
	}

	decoded, mapErr := toMap(jsonData)
	if mapErr != nil {
		common.Throw(k.vu.Runtime(), mapErr)
		return nil
	}
	return decoded
}
