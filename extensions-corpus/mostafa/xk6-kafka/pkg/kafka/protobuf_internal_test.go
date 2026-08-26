package kafka

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const depSubjectName = "dep-subject"

func TestNormalizeProtobufFormat(t *testing.T) {
	assert.Equal(t, "object", normalizeProtobufFormat(""))
	assert.Equal(t, "object", normalizeProtobufFormat(" OBJECT "))
	assert.Equal(t, "bytes", normalizeProtobufFormat("bytes"))
}

func TestParseProtobufBytesInput(t *testing.T) {
	raw := []byte{1, 2, 3}
	out, err := parseProtobufBytesInput(raw)
	require.Nil(t, err)
	assert.Equal(t, raw, out)

	out, err = parseProtobufBytesInput([]any{1.0, 2.0, 3.0})
	require.Nil(t, err)
	assert.Equal(t, raw, out)

	base64Data := base64.StdEncoding.EncodeToString(raw)
	out, err = parseProtobufBytesInput(base64Data)
	require.Nil(t, err)
	assert.Equal(t, raw, out)

	_, err = parseProtobufBytesInput("not-base64")
	require.Error(t, err)
	assert.Equal(t, ErrProtobufUnsupportedFormatInput, err)

	_, err = parseProtobufBytesInput([]any{1.0, "x"})
	require.Error(t, err)
	assert.Equal(t, ErrProtobufUnsupportedFormatInput, err)
}

func TestEncodeDecodeMessageIndexes(t *testing.T) {
	encoded := encodeProtobufMessageIndexes([]int{0})
	assert.Equal(t, []byte{0}, encoded)

	encoded = encodeProtobufMessageIndexes([]int{1, 2, 3})
	read, indexes, err := parseProtobufMessageIndexes(encoded)
	require.Nil(t, err)
	assert.Equal(t, len(encoded), read)
	assert.Equal(t, []int{1, 2, 3}, indexes)
}

func TestParseProtobufFileDescriptorAndMessageResolution(t *testing.T) {
	schema := &Schema{
		Schema: `syntax = "proto3"; package t; message A { string v = 1; }`,
	}
	fileDesc, err := parseProtobufFileDescriptor(schema)
	require.Nil(t, err)
	require.NotNil(t, fileDesc)

	messageDesc, msgErr := resolveTargetProtobufMessage(fileDesc, "t.A")
	require.Nil(t, msgErr)
	require.NotNil(t, messageDesc)

	_, msgErr = resolveTargetProtobufMessage(fileDesc, "t.Missing")
	require.Error(t, msgErr)
	assert.Equal(t, ErrProtobufMissingMessageName, msgErr)
}

func TestBuildProtobufRuntimeAmbiguousMessage(t *testing.T) {
	schema := &Schema{
		Schema: `syntax = "proto3"; message A { string v = 1; } message B { string v = 1; }`,
	}
	_, err := buildProtobufRuntime(schema)
	require.Error(t, err)
	assert.Equal(t, ErrProtobufAmbiguousMessageName, err)
}

func TestToMessageDescriptorInvalidIndexPath(t *testing.T) {
	schema := &Schema{
		Schema: `syntax = "proto3"; package t; message A { string v = 1; }`,
	}
	fileDesc, err := parseProtobufFileDescriptor(schema)
	require.Nil(t, err)

	_, descErr := toMessageDescriptor(fileDesc, []int{99})
	require.Error(t, descErr)
	assert.Equal(t, ErrProtobufInvalidMessageIndexPath, descErr)
}

func TestBuildProtobufDependencyMapWithReferences(t *testing.T) {
	protobufType := Protobuf
	referenced := &Schema{
		Schema:       `syntax = "proto3"; package dep; message Dep { string value = 1; }`,
		SchemaType:   &protobufType,
		Subject:      depSubjectName,
		References:   nil,
		Dependencies: nil,
	}
	root := &Schema{
		Schema: `syntax = "proto3"; import "dep.proto"; message Root { dep.Dep value = 1; }`,
		References: []Reference{
			{Name: "dep.proto", Subject: depSubjectName, Version: 1},
		},
		resolver: func(name string) (*Schema, error) {
			if name == "dep.proto" || name == depSubjectName {
				return referenced, nil
			}
			return nil, ErrReferenceNotFound
		},
	}

	deps, err := buildProtobufDependencyMap(root)
	require.Nil(t, err)
	assert.Contains(t, deps, "dep.proto")
}

func TestBuildProtobufDependencyMapErrorWithoutResolver(t *testing.T) {
	_, err := buildProtobufDependencyMap(&Schema{
		Schema: `syntax = "proto3"; import "dep.proto"; message Root { string v = 1; }`,
		References: []Reference{
			{Name: "dep.proto", Subject: "dep-subject", Version: 1},
		},
	})
	require.Error(t, err)
	assert.Equal(t, failedResolveReferences, err.Code)
}

func TestCollectProtobufReferenceSchemasVisitedSkipsReprocessing(t *testing.T) {
	ref := &Schema{Schema: `syntax = "proto3"; package dep; message Dep { string value = 1; }`}
	root := &Schema{
		References: []Reference{
			{Name: "dep.proto", Subject: "dep-subject", Version: 1},
		},
		resolver: func(name string) (*Schema, error) {
			if name == "dep.proto" || name == "dep-subject" {
				return ref, nil
			}
			return nil, ErrReferenceNotFound
		},
	}

	out := map[string]string{"dep.proto": "existing"}
	visited := map[string]bool{"dep.proto": true}
	err := collectProtobufReferenceSchemas(root, out, visited)
	require.Nil(t, err)
	assert.Equal(t, "existing", out["dep.proto"])
}

func TestParseProtobufFileDescriptorRejectsEmptySchema(t *testing.T) {
	_, err := parseProtobufFileDescriptor(&Schema{Schema: " "})
	require.Error(t, err)
	assert.Equal(t, ErrProtobufSchemaCompileFailed, err)
}

func TestProtobufSerdeSerializeDeserialize(t *testing.T) {
	serde := &ProtobufSerde{}
	schema := &Schema{
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	encoded, err := serde.Serialize(map[string]any{"v": "ok"}, schema)
	require.Nil(t, err)
	require.NotEmpty(t, encoded)

	decoded, decodeErr := serde.Deserialize(encoded, schema)
	require.Nil(t, decodeErr)
	object, ok := decoded.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", object["v"])
}

func TestProtobufSerdeSerializeErrorPaths(t *testing.T) {
	serde := &ProtobufSerde{}
	schema := &Schema{
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	_, err := serde.Serialize(make(chan int), schema)
	require.Error(t, err)
	assert.Equal(t, ErrProtobufObjectValidationFailed, err)

	_, err = serde.Serialize(map[string]any{"unknown": "x"}, schema)
	require.Error(t, err)
	assert.Equal(t, failedToEncode, err.Code)
}

func TestProtobufSerdeDeserializeErrorPaths(t *testing.T) {
	serde := &ProtobufSerde{}
	schema := &Schema{
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	_, err := serde.Deserialize([]byte{255, 1, 2}, schema)
	require.Error(t, err)
	assert.Equal(t, failedToDecodeFromBinary, err.Code)
}

func TestToMessageIndexArrayAndDescriptorNestedPaths(t *testing.T) {
	schema := &Schema{
		Schema: `
syntax = "proto3";
package t;
message A { string v = 1; }
message B {
  message C { string v = 1; }
  C nested = 1;
}
`,
	}
	fileDesc, err := parseProtobufFileDescriptor(schema)
	require.Nil(t, err)

	// Second top-level message forces recursive toMessageIndexes path.
	b := fileDesc.Messages().Get(1)
	assert.Equal(t, []int{1}, toMessageIndexArray(b))

	// Nested message exercises both file and message descriptor resolution branches.
	c, descErr := toMessageDescriptor(fileDesc, []int{1, 0})
	require.Nil(t, descErr)
	assert.Equal(t, protoreflect.FullName("t.B.C"), c.FullName())

	// Resolve from a message descriptor (not file descriptor) branch.
	c2, descErr := toMessageDescriptor(b, []int{0})
	require.Nil(t, descErr)
	assert.Equal(t, protoreflect.FullName("t.B.C"), c2.FullName())

	// Empty path on message descriptor is invalid for current traversal logic.
	_, descErr = toMessageDescriptor(b, []int{})
	require.Error(t, descErr)
	assert.Equal(t, ErrProtobufInvalidMessageIndexPath, descErr)
}

func TestResolveReferenceByNameFallbackAndNotFound(t *testing.T) {
	referenced := &Schema{Schema: `syntax = "proto3"; package dep; message Dep { string value = 1; }`}
	schema := &Schema{
		References: []Reference{
			{Name: "dep.proto", Subject: "dep-subject", Version: 1},
		},
		resolver: func(name string) (*Schema, error) {
			if name == "dep-subject" {
				return referenced, nil
			}
			return nil, ErrReferenceNotFound
		},
	}

	resolved, err := resolveReferenceByName(schema, "dep.proto")
	require.Nil(t, err)
	assert.Equal(t, referenced, resolved)

	_, err = resolveReferenceByName(schema, "missing.proto")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrReferenceNotFound)
}

func TestSerializeDeserializeProtobufErrorPaths(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	validSchema := &Schema{
		ID:          1,
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	requireGoErrorMessage(t, func() {
		test.module.serializeProtobuf(&Container{
			Data:           map[string]any{"v": "x"},
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "invalid",
		})
	}, "Unsupported input type for protobufFormat")

	requireGoErrorMessage(t, func() {
		test.module.serializeProtobuf(&Container{
			Data:           "not-base64",
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "bytes",
		})
	}, "Unsupported input type for protobufFormat")

	requireGoErrorMessage(t, func() {
		test.module.deserializeProtobuf(&Container{
			Data:           []byte{0, 0, 0, 0, 1},
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "object",
		})
	}, "Invalid protobuf message-index path")
}

func TestDeserializeProtobufFormatsAndBranches(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	validSchema := &Schema{
		ID:          7,
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	wire := test.module.serializeProtobuf(&Container{
		Data:           map[string]any{"v": "ok"},
		Schema:         validSchema,
		SchemaType:     Protobuf,
		ProtobufFormat: "object",
	})
	require.NotEmpty(t, wire)

	// bytes mode should return raw protobuf payload for []byte input.
	decodedBytes := test.module.deserializeProtobuf(&Container{
		Data:           wire,
		Schema:         validSchema,
		SchemaType:     Protobuf,
		ProtobufFormat: "bytes",
	})
	require.IsType(t, []byte{}, decodedBytes)

	// object mode should decode from base64 input too.
	decodedObject := test.module.deserializeProtobuf(&Container{
		Data:           base64.StdEncoding.EncodeToString(wire),
		Schema:         validSchema,
		SchemaType:     Protobuf,
		ProtobufFormat: "object",
	})
	asMap, ok := decodedObject.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ok", asMap["v"])

	requireGoErrorMessage(t, func() {
		test.module.deserializeProtobuf(&Container{
			Data:           123,
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "object",
		})
	}, "Unsupported input type for protobufFormat")

	requireGoErrorMessage(t, func() {
		test.module.deserializeProtobuf(&Container{
			Data:           "plain-text",
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "object",
		})
	}, "Invalid message: invalid start byte.")
}

func TestDeserializeProtobufParseAndPayloadFailures(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	validSchema := &Schema{
		ID:          11,
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	runtime, err := buildProtobufRuntime(validSchema)
	require.Nil(t, err)

	invalidPayloadWire := test.module.encodeProtobufWireFormat([]byte{255, 1, 2}, validSchema.ID, runtime.indexes)
	requireGoErrorContains(t, func() {
		test.module.deserializeProtobuf(&Container{
			Data:           invalidPayloadWire,
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "object",
		})
	}, "Failed to decode protobuf payload")

	badSchema := &Schema{
		Schema: `syntax = "proto3"; package t; import "missing.proto"; message A { string v = 1; }`,
	}
	expectedError := "Missing protobuf import or dependency, " +
		"OriginalError: .:1:38: could not resolve path \"missing.proto\": file does not exist"
	requireGoErrorMessage(t, func() {
		test.module.deserializeProtobuf(&Container{
			Data:           invalidPayloadWire,
			Schema:         badSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "object",
		})
	}, expectedError)
}

func TestSerializeProtobufBytesValidationFailure(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	validSchema := &Schema{
		Schema:      `syntax = "proto3"; package t; message A { string v = 1; }`,
		MessageName: "t.A",
	}

	requireGoErrorContains(t, func() {
		test.module.serializeProtobuf(&Container{
			Data:           []byte{255, 1, 2},
			Schema:         validSchema,
			SchemaType:     Protobuf,
			ProtobufFormat: "bytes",
		})
	}, "Failed to validate protobuf binary input")
}

func requireGoErrorContains(t *testing.T, fn func(), expectedSubstring string) {
	t.Helper()

	defer func() {
		err := recover()
		require.NotNil(t, err)

		errObj, ok := err.(*sobek.Object)
		require.True(t, ok)
		actual := errObj.ToString().String()
		require.True(t, strings.Contains(actual, expectedSubstring), "expected %q to contain %q", actual, expectedSubstring)
	}()

	fn()
}

// benchProtobufSchema is a small but non-trivial schema covering a custom
// dependency and a well-known import. It is shared across the protobuf
// benchmarks below so they exercise the same compile cost.
func benchProtobufSchema() *Schema {
	return &Schema{
		Schema: `syntax = "proto3";
package bench;
import "google/protobuf/timestamp.proto";
import "bench/inner.proto";
message Outer {
  string name = 1;
  google.protobuf.Timestamp ts = 2;
  bench.Inner inner = 3;
}`,
		MessageName: "bench.Outer",
		Dependencies: map[string]string{
			"bench/inner.proto": `syntax = "proto3";
package bench;
message Inner {
  int64 id = 1;
  repeated string tags = 2;
}`,
		},
	}
}

func TestParseProtobufFileDescriptorCachesByContent(t *testing.T) {
	// Different *Schema pointers, identical content — mirrors the JS path,
	// where decodeArgument allocates a fresh *Schema on every serialize()
	// call (see TestDecodeArgumentAllocatesFreshSchemaPerCall).
	src := `syntax = "proto3"; package cachebycontent; message M { string v = 1; }`
	s1 := &Schema{Schema: src, MessageName: "cachebycontent.M"}
	s2 := &Schema{Schema: src, MessageName: "cachebycontent.M"}
	require.NotSame(t, s1, s2)

	fd1, err := parseProtobufFileDescriptor(s1)
	require.Nil(t, err)
	fd2, err := parseProtobufFileDescriptor(s2)
	require.Nil(t, err)
	assert.Same(t, fd1, fd2, "identical schema content must yield the same cached FileDescriptor across *Schema instances")
}

func TestParseProtobufFileDescriptorDistinctContentDoesNotCollide(t *testing.T) {
	a := &Schema{Schema: `syntax = "proto3"; package distinctcache; message A { string v = 1; }`}
	b := &Schema{Schema: `syntax = "proto3"; package distinctcache; message B { int32 n = 1; }`}

	fdA, err := parseProtobufFileDescriptor(a)
	require.Nil(t, err)
	fdB, err := parseProtobufFileDescriptor(b)
	require.Nil(t, err)
	assert.NotSame(t, fdA, fdB)
}

func TestParseProtobufFileDescriptorCachesCompileError(t *testing.T) {
	// Unique import name so the cache enters from a known-miss state
	// regardless of test ordering or parallel sibling-test pollution.
	schema := &Schema{
		Schema: `syntax = "proto3"; package errorcache; import "errorcache_missing_xK6.proto"; message A { string v = 1; }`,
	}

	_, firstErr := parseProtobufFileDescriptor(schema)
	require.Error(t, firstErr)
	_, secondErr := parseProtobufFileDescriptor(schema)
	require.Error(t, secondErr)
	// Failed compiles are cached so pathological schemas do not re-run
	// protocompile.Compile on every retry.
	assert.Same(t, firstErr, secondErr)
}

func TestParseProtobufFileDescriptorConcurrentInit(t *testing.T) {
	src := `syntax = "proto3"; package concurrentinit; message Race { string v = 1; }`
	const workers = 16

	// Distinct *Schema instances with identical content — the JS-path shape
	// under concurrent VUs.
	schemas := make([]*Schema, workers)
	for i := range schemas {
		schemas[i] = &Schema{Schema: src, MessageName: "concurrentinit.Race"}
	}

	results := make([]protoreflect.FileDescriptor, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(idx int) {
			defer wg.Done()
			fd, err := parseProtobufFileDescriptor(schemas[idx])
			if err != nil {
				t.Errorf("worker %d: parseProtobufFileDescriptor returned error: %v", idx, err)
				return
			}
			results[idx] = fd
		}(i)
	}
	wg.Wait()

	for i := 1; i < workers; i++ {
		assert.Same(t, results[0], results[i])
	}
}

// TestDecodeArgumentAllocatesFreshSchemaPerCall is the load-bearing proof
// that a per-*Schema cache cannot hit across JS-facing serialize() calls:
// decodeArgument round-trips the argument through json.Marshal/Unmarshal,
// which fabricates a new *Schema on every call even when the JS-side value
// is the identical object. Any descriptor cache that wants to amortise
// protocompile.Compile across the JS hot path must key on schema content,
// not on *Schema identity.
func TestDecodeArgumentAllocatesFreshSchemaPerCall(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	// Single sobek.Value representing the JS-side argument to
	// schemaRegistry.serialize(). The reference is reused across both
	// decodeArgument calls below — that is the JS-author's perspective of
	// "passing the same schema object every iteration."
	jsArg := test.rt.ToValue(map[string]any{
		"data": "x",
		"schema": map[string]any{
			"schema":      `syntax = "proto3"; package t; message A { string v = 1; }`,
			"messageName": "t.A",
		},
		"schemaType": string(Protobuf),
	})

	var first, second Container
	decodeArgument(test.rt, jsArg, &first, "test")
	decodeArgument(test.rt, jsArg, &second, "test")

	require.NotNil(t, first.Schema)
	require.NotNil(t, second.Schema)
	// Different pointers despite the same JS-side input — this is the
	// architectural reason any cache must key on schema content, not on
	// *Schema identity.
	assert.NotSame(t, first.Schema, second.Schema,
		"decodeArgument must allocate a fresh *Schema per call; if this ever changes, per-instance caching becomes viable")

	// The content fields, however, survive the round-trip intact — which is
	// what makes a content-keyed cache feasible.
	assert.Equal(t, first.Schema.Schema, second.Schema.Schema)
	assert.Equal(t, first.Schema.MessageName, second.Schema.MessageName)
}

// BenchmarkBuildProtobufRuntime exercises the descriptor compilation path
// invoked on every Serialize/Deserialize call. Each iteration calls
// buildProtobufRuntime with the same *Schema instance, so a correct
// implementation should not recompile the descriptor graph beyond the first
// iteration. Run with `-benchmem` to surface the per-call allocations.
func BenchmarkBuildProtobufRuntime(b *testing.B) {
	schema := benchProtobufSchema()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		runtime, err := buildProtobufRuntime(schema)
		if err != nil {
			b.Fatalf("buildProtobufRuntime returned error: %v", err)
		}
		if runtime == nil || runtime.messageDesc == nil {
			b.Fatalf("buildProtobufRuntime returned empty runtime")
		}
	}
}

// BenchmarkProtobufSerdeSerializeViaDecodeArgument mirrors the JS-facing
// hot path: each iteration runs decodeArgument (the json.Marshal/Unmarshal
// round-trip the serialize handler performs on every call) before invoking
// Serialize, so the *Schema is freshly allocated each time. Compared with
// BenchmarkProtobufSerdeSerialize — which reuses one *Schema — this proves
// that a cache keyed on *Schema pointer identity cannot hit the JS path.
func BenchmarkProtobufSerdeSerializeViaDecodeArgument(b *testing.B) {
	test := getTestModuleInstance(b)
	test.moveToVUCode()
	serde := &ProtobufSerde{}

	schema := benchProtobufSchema()
	jsArg := test.rt.ToValue(map[string]any{
		"data": map[string]any{
			"name": "load-test",
			"ts":   "2026-01-01T00:00:00Z",
			"inner": map[string]any{
				"id":   "42",
				"tags": []any{"a", "b"},
			},
		},
		"schema": map[string]any{
			"schema":       schema.Schema,
			"messageName":  schema.MessageName,
			"dependencies": schema.Dependencies,
		},
		"schemaType": string(Protobuf),
	})

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var c Container
		decodeArgument(test.rt, jsArg, &c, "test")
		encoded, err := serde.Serialize(c.Data, c.Schema)
		if err != nil {
			b.Fatalf("Serialize returned error: %v", err)
		}
		if len(encoded) == 0 {
			b.Fatalf("Serialize returned empty payload")
		}
	}
}

// BenchmarkProtobufSerdeSerialize exercises the full Serialize hot path used
// by xk6-kafka's JS-facing schemaRegistry.serialize. It is the same shape as
// what an in-cluster k6 script calls on every produce() iteration; without a
// descriptor cache the per-call cost grows in lockstep with descriptor graph
// size.
func BenchmarkProtobufSerdeSerialize(b *testing.B) {
	serde := &ProtobufSerde{}
	schema := benchProtobufSchema()
	data := map[string]any{
		"name": "load-test",
		"ts":   "2026-01-01T00:00:00Z",
		"inner": map[string]any{
			"id":   "42",
			"tags": []any{"a", "b"},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		encoded, err := serde.Serialize(data, schema)
		if err != nil {
			b.Fatalf("Serialize returned error: %v", err)
		}
		if len(encoded) == 0 {
			b.Fatalf("Serialize returned empty payload")
		}
	}
}
