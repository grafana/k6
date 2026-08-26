package kafka

import (
	"testing"

	"github.com/hamba/avro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPrimitiveTypeName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "null", getPrimitiveTypeName(avro.Null))
	assert.Equal(t, "boolean", getPrimitiveTypeName(avro.Boolean))
	assert.Equal(t, "int", getPrimitiveTypeName(avro.Int))
	assert.Equal(t, "long", getPrimitiveTypeName(avro.Long))
	assert.Equal(t, "float", getPrimitiveTypeName(avro.Float))
	assert.Equal(t, "double", getPrimitiveTypeName(avro.Double))
	assert.Equal(t, "bytes", getPrimitiveTypeName(avro.Bytes))
	assert.Equal(t, "string", getPrimitiveTypeName(avro.String))
	assert.Equal(t, "", getPrimitiveTypeName(avro.Record))
	assert.Equal(t, "", getPrimitiveTypeName(avro.Type("unknown-primitive")))
}

func TestGetPrimitiveUnionDiscriminator(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", getPrimitiveUnionDiscriminator(nil))

	schema, err := avro.Parse(`{"type":"int"}`)
	require.NoError(t, err)
	assert.Equal(t, "int", getPrimitiveUnionDiscriminator(schema))

	logical, err := avro.Parse(`{"type":"int","logicalType":"date"}`)
	require.NoError(t, err)
	assert.Equal(t, "int.date", getPrimitiveUnionDiscriminator(logical))
}

func TestConvertNumericValueToByte(t *testing.T) {
	t.Parallel()
	b, err := convertNumericValueToByte(float64(42))
	require.NoError(t, err)
	assert.Equal(t, byte(42), b)

	b, err = convertNumericValueToByte(int(7))
	require.NoError(t, err)
	assert.Equal(t, byte(7), b)

	b, err = convertNumericValueToByte(int32(8))
	require.NoError(t, err)
	assert.Equal(t, byte(8), b)

	b, err = convertNumericValueToByte(int64(9))
	require.NoError(t, err)
	assert.Equal(t, byte(9), b)

	_, err = convertNumericValueToByte(float64(1.5))
	require.Error(t, err)

	_, err = convertNumericValueToByte(float64(-1))
	require.Error(t, err)

	_, err = convertNumericValueToByte("x")
	require.Error(t, err)
}

func TestConvertPrimitiveTypeIntAndLong(t *testing.T) {
	t.Parallel()
	intSchema, err := avro.Parse(`{"type":"int"}`)
	require.NoError(t, err)

	out, err := convertPrimitiveType(float64(100), intSchema)
	require.NoError(t, err)
	assert.Equal(t, int32(100), out)

	_, err = convertPrimitiveType(float64(1.5), intSchema)
	require.Error(t, err)

	longSchema, err := avro.Parse(`{"type":"long"}`)
	require.NoError(t, err)
	out, err = convertPrimitiveType(float64(1<<40), longSchema)
	require.NoError(t, err)
	assert.Equal(t, int64(1<<40), out)
}

func TestConvertPrimitiveTypeBytes(t *testing.T) {
	t.Parallel()
	bytesSchema, err := avro.Parse(`{"type":"bytes"}`)
	require.NoError(t, err)

	out, err := convertPrimitiveType([]any{float64(65), float64(66)}, bytesSchema)
	require.NoError(t, err)
	assert.Equal(t, []byte("AB"), out)

	raw := []byte{1, 2}
	out, err = convertPrimitiveType(raw, bytesSchema)
	require.NoError(t, err)
	assert.Equal(t, raw, out)
}

func TestIsValueCompatibleWithSchema(t *testing.T) {
	t.Parallel()
	assert.False(t, isValueCompatibleWithSchema(nil, nil))

	nullSchema, err := avro.Parse(`{"type":"null"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema(nil, nullSchema))
	assert.False(t, isValueCompatibleWithSchema("x", nullSchema))

	intSchema, err := avro.Parse(`{"type":"int"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema(int32(1), intSchema))
	assert.False(t, isValueCompatibleWithSchema(float64(1), intSchema))

	longSchema, err := avro.Parse(`{"type":"long"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema(int64(2), longSchema))

	strSchema, err := avro.Parse(`{"type":"string"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema("hi", strSchema))
	assert.False(t, isValueCompatibleWithSchema(1, strSchema))

	bytesSchema, err := avro.Parse(`{"type":"bytes"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema([]byte{1}, bytesSchema))
	assert.False(t, isValueCompatibleWithSchema([]any{1.0}, bytesSchema))

	arrSchema, err := avro.Parse(`{"type":"array","items":"string"}`)
	require.NoError(t, err)
	assert.True(t, isValueCompatibleWithSchema([]any{"a"}, arrSchema))
	assert.False(t, isValueCompatibleWithSchema(map[string]any{}, arrSchema))
}

func TestConvertUnionFieldNil(t *testing.T) {
	t.Parallel()
	schema, err := avro.Parse(`["null","string"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	got, err := convertUnionField(nil, unionSchema)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestConvertUnionFieldPrimitiveInt(t *testing.T) {
	t.Parallel()
	schema, err := avro.Parse(`["null","int"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	got, err := convertUnionField(float64(42), unionSchema)
	require.NoError(t, err)
	assert.Equal(t, int32(42), got)
}
