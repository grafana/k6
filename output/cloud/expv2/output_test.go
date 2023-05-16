package expv2

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/lib/testutils"
	"gopkg.in/guregu/null.v3"
)

func TestNew(t *testing.T) {
	t.Parallel()

	logger, hook := testutils.NewLoggerWithHook(t)
	o, err := New(logger, cloudapi.Config{APIVersion: null.IntFrom(99)})
	require.NoError(t, err)
	require.NotNil(t, o)

	// assert the prefixed logging
	o.logger.Info("aaa")
	loglines := hook.Drain()
	require.Len(t, loglines, 1)
	assert.Equal(t, loglines[0].Data["output"], "cloudv2")

	// assert the config set
	assert.Equal(t, int64(99), o.config.APIVersion.Int64)
}

func TestOutputSetReferenceID(t *testing.T) {
	t.Parallel()
	o := Output{}
	o.SetReferenceID("my-test-run-id")
	assert.Equal(t, "my-test-run-id", o.referenceID)
}

func TestOutputSetTestRunStopCallback(t *testing.T) {
	t.Parallel()
	called := false
	o := Output{}
	o.SetTestRunStopCallback(func(e error) {
		assert.EqualError(t, e, "my new fake error")
		called = true
	})
	o.testStopFunc(errors.New("my new fake error"))
	assert.True(t, called)
}
