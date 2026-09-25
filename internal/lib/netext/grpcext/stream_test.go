package grpcext

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestConvertNullProtobufValue(t *testing.T) {
	t.Parallel()

	v, err := structpb.NewValue(nil)
	require.NoError(t, err)

	msg := dynamicpb.NewMessage(v.ProtoReflect().Descriptor())
	proto.Merge(msg, v)

	got, err := convert(protojson.MarshalOptions{EmitUnpopulated: true}, msg)
	require.NoError(t, err)
	require.Nil(t, got)
}
