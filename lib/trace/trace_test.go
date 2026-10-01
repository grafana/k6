package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndRegisteredNames(t *testing.T) {
	t.Parallel()

	Register("test-register-a")
	Register("test-register-b")

	names := RegisteredNames()
	assert.Contains(t, names, "test-register-a")
	assert.Contains(t, names, "test-register-b")
}

func TestRegisterDuplicatePanics(t *testing.T) {
	t.Parallel()

	Register("test-register-dup")
	assert.Panics(t, func() { Register("test-register-dup") })
}

func TestParseSetEmptyAndNone(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "none"} {
		set, err := ParseSet(raw)
		require.NoError(t, err)
		assert.False(t, set.Any())
		assert.False(t, set.Enabled("anything"))
	}
}

func TestParseSetAll(t *testing.T) {
	t.Parallel()

	Register("test-all-a")
	Register("test-all-b")

	set, err := ParseSet("all")
	require.NoError(t, err)
	assert.True(t, set.Any())
	assert.True(t, set.Enabled("test-all-a"))
	assert.True(t, set.Enabled("test-all-b"))
}

func TestParseSetList(t *testing.T) {
	t.Parallel()

	Register("test-list-a")
	Register("test-list-b")
	Register("test-list-c")

	set, err := ParseSet("test-list-a,test-list-b")
	require.NoError(t, err)
	assert.True(t, set.Any())
	assert.True(t, set.Enabled("test-list-a"))
	assert.True(t, set.Enabled("test-list-b"))
	assert.False(t, set.Enabled("test-list-c"))
}

func TestParseSetErrors(t *testing.T) {
	t.Parallel()

	Register("test-errors-a")

	tests := map[string]struct {
		raw     string
		wantErr error
	}{
		"unknown module":       {"test-errors-unknown", ErrUnknownModule},
		"all mixed with name":  {"all,test-errors-a", ErrInvalidTracing},
		"none mixed with name": {"none,test-errors-a", ErrInvalidTracing},
		"trailing comma":       {"test-errors-a,", ErrInvalidTracing},
		"leading comma":        {",test-errors-a", ErrInvalidTracing},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseSet(tc.raw)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestSetZeroValue(t *testing.T) {
	t.Parallel()

	var set Set
	assert.False(t, set.Any())
	assert.False(t, set.Enabled("anything"))
}
