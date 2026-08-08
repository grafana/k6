package grpcext

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestConvertMatchesProtoJSON(t *testing.T) {
	t.Parallel()

	file := compileTestFile(t, "convert_test.proto", `
syntax = "proto3";
package converttest;

import "google/protobuf/any.proto";
import "google/protobuf/duration.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/struct.proto";

message Nested {
  int32 count = 1;
  repeated string tags = 2;
}

enum State {
  STATE_UNSPECIFIED = 0;
  STATE_READY = 1;
}

message Envelope {
  string snake_case = 1;
  int32 int32_value = 2;
  int64 int64_value = 3;
  uint64 uint64_value = 4;
  float float_value = 5;
  double double_value = 6;
  bytes bytes_value = 7;
  bool bool_value = 8;
  State state = 9;
  repeated int64 values = 10;
  map<string, int64> labels = 11;
  map<bool, Nested> flags = 12;
  Nested child = 13;
  oneof selection {
    string name = 14;
    int32 id = 15;
  }
  optional string optional_value = 16;
  google.protobuf.Timestamp timestamp = 17;
  google.protobuf.Duration duration = 18;
  google.protobuf.Value value = 19;
  google.protobuf.Any detail = 20;
  map<int32, uint64> numeric_labels = 21;
  map<string, State> states = 22;
  repeated float float_values = 23;
}
`)
	message := file.Messages().ByName("Envelope")
	nested := file.Messages().ByName("Nested")
	types := new(protoregistry.Types)
	require.NoError(t, types.RegisterMessage(dynamicpb.NewMessageType(nested)))

	complexMessage := messageFromJSON(t, message, `{
  "snakeCase": "response",
  "int32Value": 7,
  "int64Value": "9007199254740993",
  "uint64Value": "18446744073709551615",
  "floatValue": "NaN",
  "doubleValue": "-Infinity",
  "bytesValue": "AQI=",
  "boolValue": true,
  "state": "STATE_READY",
  "values": ["1", "2"],
  "labels": {"first": "3", "second": "4"},
  "flags": {"true": {"count": 5, "tags": ["nested"]}},
  "child": {"count": 6},
  "name": "oneof",
  "optionalValue": "present",
  "timestamp": "2024-02-03T04:05:06.007008009Z",
  "duration": "-3.004005006s",
  "value": {"items": [null, true, 1.5, "text"]},
  "detail": {
    "@type": "type.googleapis.com/converttest.Nested",
    "count": 8,
    "tags": ["any"]
  },
  "numericLabels": {"-1": "18446744073709551615"},
  "states": {"known": "STATE_READY", "unknown": 99},
  "floatValues": [0.1, 0.0000001]
}`, protojson.UnmarshalOptions{Resolver: types})
	finiteMessage := messageFromJSON(t, message, `{
  "floatValue": 0.1,
  "doubleValue": 0.0000001,
  "state": 99,
  "numericLabels": {"-1": "18446744073709551615"},
  "states": {"known": "STATE_READY", "unknown": 99},
  "floatValues": [0.1, 0.0000001]
}`, protojson.UnmarshalOptions{Resolver: types})
	replacementMessage := messageFromJSON(t, message, `{"snakeCase": "replacement \uFFFD"}`, protojson.UnmarshalOptions{Resolver: types})
	emptyMessage := dynamicpb.NewMessage(message)

	legacyFile := compileTestFile(t, "legacy_test.proto", `
syntax = "proto2";
package convertlegacy;

message Legacy {
  optional string optional_string = 1 [default = "default"];
  optional int64 optional_id = 2;
  optional Legacy child = 3;
  repeated string names = 4;
}
`)
	legacyMessage := dynamicpb.NewMessage(legacyFile.Messages().ByName("Legacy"))

	anyTimestamp := messageFromJSON(t, (&anypb.Any{}).ProtoReflect().Descriptor(), `{
  "@type": "type.googleapis.com/google.protobuf.Timestamp",
  "value": "2024-02-03T04:05:06.007008009Z"
}`, protojson.UnmarshalOptions{})
	emptyAny := dynamicpb.NewMessage((&anypb.Any{}).ProtoReflect().Descriptor())
	fieldMask := messageFromJSON(t, (&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor(), `"snakeCase,nested.value"`, protojson.UnmarshalOptions{})
	emptyFieldMask := dynamicpb.NewMessage((&fieldmaskpb.FieldMask{}).ProtoReflect().Descriptor())
	structValue := messageFromJSON(t, (&structpb.Struct{}).ProtoReflect().Descriptor(), `{"number": 1.5, "null": null, "list": [true, "text"]}`, protojson.UnmarshalOptions{})
	emptyStruct := dynamicpb.NewMessage((&structpb.Struct{}).ProtoReflect().Descriptor())
	listValue := messageFromJSON(t, (&structpb.ListValue{}).ProtoReflect().Descriptor(), `[null, true, 1.5, "text"]`, protojson.UnmarshalOptions{})
	emptyList := dynamicpb.NewMessage((&structpb.ListValue{}).ProtoReflect().Descriptor())
	knownValue := messageFromJSON(t, (&structpb.Value{}).ProtoReflect().Descriptor(), `{"nested": [null, "text"]}`, protojson.UnmarshalOptions{})
	nullValue := messageFromJSON(t, (&structpb.Value{}).ProtoReflect().Descriptor(), `null`, protojson.UnmarshalOptions{})
	timestamp := messageFromJSON(t, (&timestamppb.Timestamp{}).ProtoReflect().Descriptor(), `"2024-02-03T04:05:06.007008009Z"`, protojson.UnmarshalOptions{})
	zeroTimestamp := dynamicpb.NewMessage((&timestamppb.Timestamp{}).ProtoReflect().Descriptor())
	duration := messageFromJSON(t, (&durationpb.Duration{}).ProtoReflect().Descriptor(), `"-3.004005006s"`, protojson.UnmarshalOptions{})
	zeroDuration := dynamicpb.NewMessage((&durationpb.Duration{}).ProtoReflect().Descriptor())
	wrapper := messageFromJSON(t, (&wrapperspb.Int64Value{}).ProtoReflect().Descriptor(), `"9007199254740993"`, protojson.UnmarshalOptions{})
	zeroWrapper := dynamicpb.NewMessage((&wrapperspb.Int64Value{}).ProtoReflect().Descriptor())
	bytesWrapper := messageFromJSON(t, (&wrapperspb.BytesValue{}).ProtoReflect().Descriptor(), `"AQI="`, protojson.UnmarshalOptions{})
	empty := dynamicpb.NewMessage((&emptypb.Empty{}).ProtoReflect().Descriptor())

	tests := []struct {
		name      string
		marshaler protojson.MarshalOptions
		message   *dynamicpb.Message
	}{
		{
			name:      "complex message with populated values",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, Resolver: types},
			message:   complexMessage,
		},
		{
			name:      "finite floats and unknown enums",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, Resolver: types},
			message:   finiteMessage,
		},
		{
			name:      "replacement character",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, Resolver: types},
			message:   replacementMessage,
		},
		{
			name:      "unpopulated proto3 fields",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, Resolver: types},
			message:   emptyMessage,
		},
		{
			name:      "proto names and enum numbers",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, UseProtoNames: true, UseEnumNumbers: true, Resolver: types},
			message:   complexMessage,
		},
		{
			name:      "proto2 unpopulated fields",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, AllowPartial: true},
			message:   legacyMessage,
		},
		{
			name:      "emit default values without presence fields",
			marshaler: protojson.MarshalOptions{EmitDefaultValues: true, AllowPartial: true},
			message:   legacyMessage,
		},
		{
			name:      "unpopulated takes precedence over default values",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true, EmitDefaultValues: true, AllowPartial: true},
			message:   legacyMessage,
		},
		{
			name:      "any containing timestamp",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   anyTimestamp,
		},
		{
			name:      "empty any",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   emptyAny,
		},
		{
			name:      "field mask",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   fieldMask,
		},
		{
			name:      "empty field mask",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   emptyFieldMask,
		},
		{
			name:      "struct",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   structValue,
		},
		{
			name:      "empty struct",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   emptyStruct,
		},
		{
			name:      "list value",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   listValue,
		},
		{
			name:      "empty list value",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   emptyList,
		},
		{
			name:      "value",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   knownValue,
		},
		{
			name:      "null value",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   nullValue,
		},
		{
			name:      "timestamp",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   timestamp,
		},
		{
			name:      "zero timestamp",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   zeroTimestamp,
		},
		{
			name:      "duration",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   duration,
		},
		{
			name:      "zero duration",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   zeroDuration,
		},
		{
			name:      "wrapper",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   wrapper,
		},
		{
			name:      "zero wrapper",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   zeroWrapper,
		},
		{
			name:      "bytes wrapper",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   bytesWrapper,
		},
		{
			name:      "empty",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   empty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			want, err := referenceConvertWithJSON(tt.marshaler, tt.message)
			require.NoError(t, err)

			direct, err := convertDirect(tt.marshaler, tt.message)
			require.NoError(t, err)
			assert.Equal(t, want, direct)

			got, err := convert(tt.marshaler, tt.message)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestConvertFallsBackToProtoJSON(t *testing.T) {
	t.Parallel()

	requiredFile := compileTestFile(t, "required_test.proto", `
syntax = "proto2";
package convertrequired;

message Required {
  required string value = 1;
}

message Text {
  optional string value = 1;
}
`)
	requiredMessage := dynamicpb.NewMessage(requiredFile.Messages().ByName("Required"))
	invalidText := dynamicpb.NewMessage(requiredFile.Messages().ByName("Text"))
	invalidText.Set(invalidText.Descriptor().Fields().ByNumber(1), protoreflect.ValueOfString(string([]byte{0xff})))
	invalidValue := dynamicpb.NewMessage((&structpb.Value{}).ProtoReflect().Descriptor())
	unknownAny := dynamicpb.NewMessage((&anypb.Any{}).ProtoReflect().Descriptor())
	unknownAny.Set(
		unknownAny.Descriptor().Fields().ByNumber(1),
		protoreflect.ValueOfString("type.googleapis.com/missing.Message"),
	)

	tests := []struct {
		name      string
		marshaler protojson.MarshalOptions
		message   *dynamicpb.Message
	}{
		{
			name:      "missing required field",
			marshaler: protojson.MarshalOptions{EmitUnpopulated: true},
			message:   requiredMessage,
		},
		{
			name:      "invalid UTF-8",
			marshaler: protojson.MarshalOptions{},
			message:   invalidText,
		},
		{
			name:      "value without a oneof field",
			marshaler: protojson.MarshalOptions{},
			message:   invalidValue,
		},
		{
			name:      "unresolvable any",
			marshaler: protojson.MarshalOptions{},
			message:   unknownAny,
		},
		{
			name:      "invalid indent",
			marshaler: protojson.MarshalOptions{Indent: "x"},
			message:   dynamicpb.NewMessage((&emptypb.Empty{}).ProtoReflect().Descriptor()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected, expectedErr := convertWithJSON(tt.marshaler, tt.message)
			require.Error(t, expectedErr)

			_, directErr := convertDirect(tt.marshaler, tt.message)
			require.Error(t, directErr)

			actual, actualErr := convert(tt.marshaler, tt.message)
			assert.Equal(t, expected, actual)
			assert.EqualError(t, actualErr, expectedErr.Error())
		})
	}
}

func TestConvertPreservesNilMessageContract(t *testing.T) {
	t.Parallel()

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}
	// Conn and Stream allocate responses before conversion. A nil dynamic message remains unsupported,
	// as it was when conversion consisted only of the protojson path.
	require.Panics(t, func() {
		_, _ = referenceConvertWithJSON(marshaler, nil)
	})
	require.Panics(t, func() {
		_, _ = convert(marshaler, nil)
	})
}

func TestConvertDirectPreservesNegativeZero(t *testing.T) {
	t.Parallel()

	file := compileTestFile(t, "float_test.proto", `
syntax = "proto3";
package convertfloat;

message Floats {
  float float_value = 1;
  double double_value = 2;
}
`)
	message := messageFromJSON(t, file.Messages().ByName("Floats"), `{
  "floatValue": -0,
  "doubleValue": -0
}`, protojson.UnmarshalOptions{})
	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}

	expected, err := referenceConvertWithJSON(marshaler, message)
	require.NoError(t, err)
	direct, err := convertDirect(marshaler, message)
	require.NoError(t, err)
	assert.Equal(t, expected, direct)

	expectedValues := expected.(map[string]any)
	directValues := direct.(map[string]any)
	assert.Equal(t, math.Signbit(expectedValues["floatValue"].(float64)), math.Signbit(directValues["floatValue"].(float64)))
	assert.Equal(t, math.Signbit(expectedValues["doubleValue"].(float64)), math.Signbit(directValues["doubleValue"].(float64)))
}

// BenchmarkConnInvokeResponse lives with converter tests because it measures response conversion
// through Conn.Invoke.
func BenchmarkConnInvokeResponse(b *testing.B) {
	method := benchmarkMethodDescriptor(b)
	fields := method.Output().Fields()
	bodyField := fields.ByName("body")
	sequenceField := fields.ByName("sequence")
	request := InvokeRequest{
		Method:           "/benchmark.Conversion/Invoke",
		MethodDescriptor: method,
		Message:          []byte(`{}`),
	}

	for _, size := range []struct {
		name string
		body string
	}{
		{name: "128B", body: strings.Repeat("x", 128)},
		{name: "16KiB", body: strings.Repeat("x", 16*1024)},
	} {
		b.Run(size.name, func(b *testing.B) {
			conn := Conn{raw: invokemock(func(_ *dynamicpb.Message, out *dynamicpb.Message, _ ...grpc.CallOption) error {
				out.Set(bodyField, protoreflect.ValueOfString(size.body))
				out.Set(sequenceField, protoreflect.ValueOfInt64(9007199254740993))
				return nil
			})}

			b.ReportAllocs()
			for b.Loop() {
				response, err := conn.Invoke(context.Background(), request)
				if err != nil {
					b.Fatal(err)
				}
				if response.Message == nil {
					b.Fatal("expected converted response message")
				}
			}
		})
	}
}

func compileTestFile(tb testing.TB, filename, source string) protoreflect.FileDescriptor {
	tb.Helper()

	resolver := protocompile.WithStandardImports(&protocompile.SourceResolver{
		Accessor: protocompile.SourceAccessorFromMap(map[string]string{filename: source}),
	})
	compiler := protocompile.Compiler{Resolver: resolver}
	files, err := compiler.Compile(context.Background(), filename)
	require.NoError(tb, err)
	require.Len(tb, files, 1)
	return files[0]
}

func messageFromJSON(
	tb testing.TB,
	descriptor protoreflect.MessageDescriptor,
	input string,
	unmarshaler protojson.UnmarshalOptions,
) *dynamicpb.Message {
	tb.Helper()

	message := dynamicpb.NewMessage(descriptor)
	require.NoError(tb, unmarshaler.Unmarshal([]byte(input), message))
	return message
}

func referenceConvertWithJSON(marshaler protojson.MarshalOptions, message *dynamicpb.Message) (any, error) {
	raw, err := marshaler.Marshal(message)
	if err != nil {
		return nil, err
	}

	var converted any
	if err := json.Unmarshal(raw, &converted); err != nil {
		return nil, err
	}
	return converted, nil
}

func benchmarkMethodDescriptor(b *testing.B) protoreflect.MethodDescriptor {
	file := compileTestFile(b, "benchmark.proto", `
syntax = "proto3";
package benchmark;

service Conversion {
  rpc Invoke(Request) returns (Response);
}

message Request {}

message Response {
  string body = 1;
  int64 sequence = 2;
}
`)
	return file.Services().ByName("Conversion").Methods().ByName("Invoke")
}
