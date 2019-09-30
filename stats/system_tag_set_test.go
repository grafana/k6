package stats

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemTagSetString(t *testing.T) {
	require.Equal(t, "proto", TagProto.String())
	require.Equal(t, "subproto", TagSubProto.String())
}
