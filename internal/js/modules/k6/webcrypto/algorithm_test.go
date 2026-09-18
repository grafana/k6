package webcrypto

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAlgorithmNullish(t *testing.T) {
	t.Parallel()

	rt := sobek.New()

	tests := []struct {
		name  string
		value sobek.Value
	}{
		{name: "undefined", value: sobek.Undefined()},
		{name: "null", value: sobek.Null()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeAlgorithm(rt, tt.value, OperationIdentifierDigest)
			require.Error(t, err)

			var wcErr *Error
			require.ErrorAs(t, err, &wcErr)
			assert.Equal(t, TypeError, wcErr.Name)
			assert.Contains(t, wcErr.Message, "algorithm")
		})
	}
}
