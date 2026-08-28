package module

import (
	"context"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
)

type testVU struct {
	runtime     *sobek.Runtime
	environment map[string]string
}

func (vu testVU) Context() context.Context { return context.Background() }

func (vu testVU) Runtime() *sobek.Runtime { return vu.runtime }

func (vu testVU) LookupEnv(key string) (string, bool) {
	value, ok := vu.environment[key]
	return value, ok
}

var _ extensionapi.Environment = testVU{}

func Test_getseed(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(0), getseed(nil))

	vu := testVU{runtime: sobek.New(), environment: map[string]string{}}

	require.Equal(t, int64(0), getseed(vu))

	vu.environment["XK6_FAKER_SEED"] = "foo"

	require.Equal(t, int64(0), getseed(vu))

	vu.environment["XK6_FAKER_SEED"] = "42"

	require.Equal(t, int64(42), getseed(vu))
}
