package grpcext

// This file mirrors vendored protojson conversion. TestConvertMatchesProtoJSON differentially pins
// conversion parity against that implementation so protobuf upgrades fail the test rather than
// silently diverging.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	anyMessageName       = "google.protobuf.Any"
	durationMessageName  = "google.protobuf.Duration"
	emptyMessageName     = "google.protobuf.Empty"
	fieldMaskMessageName = "google.protobuf.FieldMask"
	listValueMessageName = "google.protobuf.ListValue"
	nullValueEnumName    = "google.protobuf.NullValue"
	structMessageName    = "google.protobuf.Struct"
	timestampMessageName = "google.protobuf.Timestamp"
	valueMessageName     = "google.protobuf.Value"
	wrapperPackageName   = "google.protobuf"

	maxDurationSeconds = 315576000000
	maxTimestampSecond = 253402300799
	minTimestampSecond = -62135596800
	maxNanos           = 999999999
)

var errDirectConversion = errors.New("cannot directly convert protobuf message")

// convert turns a dynamic response into map and slice values Sobek can expose to JavaScript.
// Sobek property access requires a real map-like value; a dynamic message can stringify correctly
// while yielding undefined properties. When enabled, EmitUnpopulated retains zero/default fields
// expected by JavaScript callers.
//
// It directly builds the values decoding protojson output would produce without a JSON buffer.
// The protojson path remains a fallback for malformed messages or unsupported descriptor behavior.
// Callers must provide an allocated message, as required by the legacy protojson conversion.
func convert(marshaler protojson.MarshalOptions, message *dynamicpb.Message) (any, error) {
	if message != nil {
		converted, err := convertDirect(marshaler, message)
		if err == nil {
			return converted, nil
		}
	}

	return convertWithJSON(marshaler, message)
}

func convertWithJSON(marshaler protojson.MarshalOptions, message *dynamicpb.Message) (any, error) {
	raw, err := marshaler.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the message: %w", err)
	}

	var converted any
	if err := json.Unmarshal(raw, &converted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal the message: %w", err)
	}

	return converted, nil
}

func convertDirect(marshaler protojson.MarshalOptions, message *dynamicpb.Message) (any, error) {
	if message == nil || strings.Trim(marshaler.Indent, " \t") != "" {
		return nil, errDirectConversion
	}

	converter := responseConverter{marshaler: marshaler}
	converted, err := converter.message(message.ProtoReflect(), "")
	if err != nil {
		return nil, err
	}
	if !marshaler.AllowPartial {
		if err := proto.CheckInitialized(message); err != nil {
			return nil, err
		}
	}

	return converted, nil
}

type responseConverter struct {
	marshaler protojson.MarshalOptions
}

func (c responseConverter) message(message protoreflect.Message, typeURL string) (any, error) {
	if !message.IsValid() || isMessageSet(message.Descriptor()) {
		return nil, errDirectConversion
	}

	switch message.Descriptor().FullName() {
	case anyMessageName:
		return c.any(message)
	case durationMessageName:
		return c.duration(message)
	case emptyMessageName:
		return map[string]any{}, nil
	case fieldMaskMessageName:
		return c.fieldMask(message)
	case listValueMessageName:
		field := message.Descriptor().Fields().ByNumber(1)
		if field == nil {
			return nil, errDirectConversion
		}
		return c.list(message.Get(field), field)
	case structMessageName:
		field := message.Descriptor().Fields().ByNumber(1)
		if field == nil {
			return nil, errDirectConversion
		}
		return c.mapValue(message.Get(field), field)
	case timestampMessageName:
		return c.timestamp(message)
	case valueMessageName:
		return c.value(message)
	}

	if isWrapperMessage(message.Descriptor().FullName()) {
		field := message.Descriptor().Fields().ByNumber(1)
		return c.singular(message.Get(field), field)
	}

	return c.object(message, typeURL)
}

// any mirrors protojson's marshalAny behavior.
func (c responseConverter) any(message protoreflect.Message) (any, error) {
	fields := message.Descriptor().Fields()
	typeField := fields.ByNumber(1)
	valueField := fields.ByNumber(2)
	if typeField == nil || valueField == nil {
		return nil, errDirectConversion
	}

	if !message.Has(typeField) {
		if !message.Has(valueField) {
			return map[string]any{}, nil
		}
		return nil, errDirectConversion
	}

	typeURL := message.Get(typeField).String()
	if !utf8.ValidString(typeURL) {
		return nil, errDirectConversion
	}

	resolver := c.resolver()
	embeddedType, err := resolver.FindMessageByURL(typeURL)
	if err != nil {
		return nil, errDirectConversion
	}

	embedded := embeddedType.New()
	if err := (proto.UnmarshalOptions{AllowPartial: true, Resolver: resolver}).Unmarshal(
		message.Get(valueField).Bytes(), embedded.Interface(),
	); err != nil {
		return nil, errDirectConversion
	}

	converted, err := c.message(embedded, typeURL)
	if err != nil {
		return nil, err
	}
	if isWellKnownMessage(embedded.Descriptor().FullName()) {
		return map[string]any{"@type": typeURL, "value": converted}, nil
	}

	object, ok := converted.(map[string]any)
	if !ok {
		return nil, errDirectConversion
	}
	return object, nil
}

// duration mirrors protojson's marshalDuration behavior.
func (c responseConverter) duration(message protoreflect.Message) (any, error) {
	fields := message.Descriptor().Fields()
	secondsField := fields.ByNumber(1)
	nanosField := fields.ByNumber(2)
	if secondsField == nil || nanosField == nil {
		return nil, errDirectConversion
	}

	seconds := message.Get(secondsField).Int()
	nanos := message.Get(nanosField).Int()
	if seconds < -maxDurationSeconds || seconds > maxDurationSeconds || nanos < -maxNanos || nanos > maxNanos {
		return nil, errDirectConversion
	}
	if (seconds > 0 && nanos < 0) || (seconds < 0 && nanos > 0) {
		return nil, errDirectConversion
	}

	sign := ""
	if seconds < 0 || nanos < 0 {
		sign, seconds, nanos = "-", -seconds, -nanos
	}
	formatted := fmt.Sprintf("%s%d.%09d", sign, seconds, nanos)
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, ".000")
	return formatted + "s", nil
}

// fieldMask mirrors protojson's marshalFieldMask behavior.
func (c responseConverter) fieldMask(message protoreflect.Message) (any, error) {
	field := message.Descriptor().Fields().ByNumber(1)
	if field == nil {
		return nil, errDirectConversion
	}

	paths := message.Get(field).List()
	converted := make([]string, 0, paths.Len())
	for i := 0; i < paths.Len(); i++ {
		path := paths.Get(i).String()
		if !protoreflect.FullName(path).IsValid() {
			return nil, errDirectConversion
		}
		camelCase := jsonCamelCase(path)
		if path != jsonSnakeCase(camelCase) {
			return nil, errDirectConversion
		}
		converted = append(converted, camelCase)
	}
	return strings.Join(converted, ","), nil
}

// timestamp mirrors protojson's marshalTimestamp behavior.
func (c responseConverter) timestamp(message protoreflect.Message) (any, error) {
	fields := message.Descriptor().Fields()
	secondsField := fields.ByNumber(1)
	nanosField := fields.ByNumber(2)
	if secondsField == nil || nanosField == nil {
		return nil, errDirectConversion
	}

	seconds := message.Get(secondsField).Int()
	nanos := message.Get(nanosField).Int()
	if seconds < minTimestampSecond || seconds > maxTimestampSecond || nanos < 0 || nanos > maxNanos {
		return nil, errDirectConversion
	}

	formatted := time.Unix(seconds, nanos).UTC().Format("2006-01-02T15:04:05.000000000")
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, "000")
	formatted = strings.TrimSuffix(formatted, ".000")
	return formatted + "Z", nil
}

// value mirrors protojson's marshalKnownValue behavior.
func (c responseConverter) value(message protoreflect.Message) (any, error) {
	oneof := message.Descriptor().Oneofs().ByName("kind")
	if oneof == nil {
		return nil, errDirectConversion
	}

	field := message.WhichOneof(oneof)
	if field == nil {
		return nil, errDirectConversion
	}
	value := message.Get(field)
	if field.Number() == 2 && (math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0)) {
		return nil, errDirectConversion
	}
	return c.singular(value, field)
}

func (c responseConverter) object(message protoreflect.Message, typeURL string) (map[string]any, error) {
	fields := c.fields(message)
	converted := make(map[string]any, len(fields)+1)
	if typeURL != "" {
		converted["@type"] = typeURL
	}

	for _, field := range fields {
		name := field.descriptor.JSONName()
		if c.marshaler.UseProtoNames {
			name = field.descriptor.TextName()
		}
		if !utf8.ValidString(name) {
			return nil, errDirectConversion
		}

		convertedValue, err := c.fieldValue(field.value, field.descriptor)
		if err != nil {
			return nil, fmt.Errorf("convert field %q: %w", name, err)
		}
		converted[name] = convertedValue
	}
	return converted, nil
}

type messageField struct {
	descriptor protoreflect.FieldDescriptor
	value      protoreflect.Value
}

// fields mirrors protojson's unpopulatedFieldRanger behavior.
func (c responseConverter) fields(message protoreflect.Message) []messageField {
	fields := make([]messageField, 0, message.Descriptor().Fields().Len())
	if c.marshaler.EmitUnpopulated || c.marshaler.EmitDefaultValues {
		descriptors := message.Descriptor().Fields()
		for i := 0; i < descriptors.Len(); i++ {
			descriptor := descriptors.Get(i)
			if message.Has(descriptor) || descriptor.ContainingOneof() != nil {
				continue
			}
			if descriptor.HasPresence() {
				if !c.marshaler.EmitUnpopulated && c.marshaler.EmitDefaultValues {
					continue
				}
				fields = append(fields, messageField{descriptor: descriptor})
				continue
			}
			fields = append(fields, messageField{descriptor: descriptor, value: message.Get(descriptor)})
		}
	}
	message.Range(func(descriptor protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		fields = append(fields, messageField{descriptor: descriptor, value: value})
		return true
	})
	// This order is load-bearing: map insertion must preserve protojson's last-write-wins behavior
	// for colliding JSON names.
	sort.Slice(fields, func(i, j int) bool {
		left, right := fields[i].descriptor, fields[j].descriptor
		if left.IsExtension() != right.IsExtension() {
			return !left.IsExtension()
		}
		if left.IsExtension() {
			return left.FullName() < right.FullName()
		}
		return left.Index() < right.Index()
	})
	return fields
}

func (c responseConverter) fieldValue(value protoreflect.Value, descriptor protoreflect.FieldDescriptor) (any, error) {
	switch {
	case descriptor.IsList():
		return c.list(value, descriptor)
	case descriptor.IsMap():
		return c.mapValue(value, descriptor)
	default:
		return c.singular(value, descriptor)
	}
}

func (c responseConverter) list(value protoreflect.Value, descriptor protoreflect.FieldDescriptor) (any, error) {
	list := value.List()
	converted := make([]any, list.Len())
	for i := 0; i < list.Len(); i++ {
		item, err := c.singular(list.Get(i), descriptor)
		if err != nil {
			return nil, err
		}
		converted[i] = item
	}
	return converted, nil
}

func (c responseConverter) mapValue(value protoreflect.Value, descriptor protoreflect.FieldDescriptor) (any, error) {
	m := value.Map()
	converted := make(map[string]any, m.Len())
	var err error
	m.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		name := key.String()
		if !utf8.ValidString(name) {
			err = errDirectConversion
			return false
		}
		convertedValue, conversionErr := c.singular(value, descriptor.MapValue())
		if conversionErr != nil {
			err = conversionErr
			return false
		}
		converted[name] = convertedValue
		return true
	})
	if err != nil {
		return nil, err
	}
	return converted, nil
}

func (c responseConverter) singular(value protoreflect.Value, descriptor protoreflect.FieldDescriptor) (any, error) {
	if !value.IsValid() {
		return nil, nil //nolint:nilnil // null is a valid protojson value
	}

	switch descriptor.Kind() {
	case protoreflect.BoolKind:
		return value.Bool(), nil
	case protoreflect.StringKind:
		if !utf8.ValidString(value.String()) {
			return nil, errDirectConversion
		}
		return value.String(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return float64(value.Int()), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return float64(value.Uint()), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Uint64Kind,
		protoreflect.Sfixed64Kind, protoreflect.Fixed64Kind:
		return value.String(), nil
	case protoreflect.FloatKind:
		return jsonFloat(value.Float(), 32)
	case protoreflect.DoubleKind:
		return jsonFloat(value.Float(), 64)
	case protoreflect.BytesKind:
		return base64.StdEncoding.EncodeToString(value.Bytes()), nil
	case protoreflect.EnumKind:
		if string(descriptor.Enum().FullName()) == nullValueEnumName {
			return nil, nil //nolint:nilnil // null is a valid protojson value
		}
		enumValue := descriptor.Enum().Values().ByNumber(value.Enum())
		if c.marshaler.UseEnumNumbers || enumValue == nil {
			return float64(value.Enum()), nil
		}
		name := string(enumValue.Name())
		if !utf8.ValidString(name) {
			return nil, errDirectConversion
		}
		return name, nil
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return c.message(value.Message(), "")
	default:
		return nil, errDirectConversion
	}
}

func (c responseConverter) resolver() interface {
	protoregistry.ExtensionTypeResolver
	protoregistry.MessageTypeResolver
} {
	if c.marshaler.Resolver != nil {
		return c.marshaler.Resolver
	}
	return protoregistry.GlobalTypes
}

func isMessageSet(descriptor protoreflect.MessageDescriptor) bool {
	messageSet, ok := descriptor.(interface{ IsMessageSet() bool })
	return ok && messageSet.IsMessageSet()
}

func isWellKnownMessage(name protoreflect.FullName) bool {
	switch name {
	case anyMessageName, durationMessageName, emptyMessageName, fieldMaskMessageName, listValueMessageName,
		structMessageName, timestampMessageName, valueMessageName:
		return true
	}
	return isWrapperMessage(name)
}

func isWrapperMessage(name protoreflect.FullName) bool {
	if name.Parent() != wrapperPackageName {
		return false
	}
	switch name.Name() {
	case "BoolValue", "Int32Value", "Int64Value", "UInt32Value", "UInt64Value", "FloatValue",
		"DoubleValue", "StringValue", "BytesValue":
		return true
	default:
		return false
	}
}

// jsonFloat mirrors protojson's internal json.Encoder.WriteFloat behavior.
func jsonFloat(value float64, bitSize int) (any, error) {
	switch {
	case math.IsNaN(value):
		return "NaN", nil
	case math.IsInf(value, 1):
		return "Infinity", nil
	case math.IsInf(value, -1):
		return "-Infinity", nil
	}

	format := byte('f')
	absolute := math.Abs(value)
	if absolute != 0 && ((bitSize == 64 && (absolute < 1e-6 || absolute >= 1e21)) ||
		(bitSize == 32 && (float32(absolute) < 1e-6 || float32(absolute) >= 1e21))) {
		format = 'e'
	}
	converted, err := strconv.ParseFloat(strconv.FormatFloat(value, format, -1, bitSize), 64)
	if err != nil {
		return nil, errDirectConversion
	}
	return converted, nil
}

// jsonCamelCase mirrors protojson's internal strs.JSONCamelCase behavior.
func jsonCamelCase(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	underscore := false
	for i := 0; i < len(value); i++ {
		character := value[i]
		if character != '_' {
			if underscore && 'a' <= character && character <= 'z' {
				character -= 'a' - 'A'
			}
			builder.WriteByte(character)
		}
		underscore = character == '_'
	}
	return builder.String()
}

// jsonSnakeCase mirrors protojson's internal strs.JSONSnakeCase behavior.
func jsonSnakeCase(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		character := value[i]
		if 'A' <= character && character <= 'Z' {
			builder.WriteByte('_')
			character += 'a' - 'A'
		}
		builder.WriteByte(character)
	}
	return builder.String()
}
