package grpc

import (
	"io"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc/metadata"
	"gopkg.in/guregu/null.v3"
)

func TestCallParamsInvalidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name        string
		JSON        string
		ErrContains string
	}{
		{
			Name:        "InvalidParam",
			JSON:        `{ void: true }`,
			ErrContains: `unknown param: "void"`,
		},
		{
			Name:        "InvalidTimeoutType",
			JSON:        `{ timeout: true }`,
			ErrContains: `invalid timeout value: unable to use type bool as a duration value`,
		},
		{
			Name:        "InvalidTimeout",
			JSON:        `{ timeout: "please" }`,
			ErrContains: `invalid duration`,
		},
		{
			Name:        "InvalidMetadata",
			JSON:        `{ metadata: "lorem" }`,
			ErrContains: `invalid metadata param: must be an object with key-value pairs`,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime, params := newParamsTestRuntime(t, tc.JSON)

			_, err := newCallParams(testRuntime.VU, params)

			assert.ErrorContains(t, err, tc.ErrContains)
		})
	}
}

func TestCallParamsMetadata(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name             string
		JSON             string
		ExpectedMetadata metadata.MD
	}{
		{
			Name:             "EmptyMetadata",
			JSON:             `{}`,
			ExpectedMetadata: metadata.New(nil),
		},
		{
			Name:             "Metadata",
			JSON:             `{metadata: {foo: "bar", baz: "qux"}}`,
			ExpectedMetadata: metadata.New(map[string]string{"foo": "bar", "baz": "qux"}),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime, params := newParamsTestRuntime(t, tc.JSON)

			p, err := newCallParams(testRuntime.VU, params)

			require.NoError(t, err)
			assert.Equal(t, tc.ExpectedMetadata, p.Metadata)
		})
	}
}

func TestCallParamsTimeOutParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name    string
		JSON    string
		Timeout time.Duration
	}{
		{
			Name:    "StringTimeout",
			JSON:    `{ timeout: "1h42m" }`,
			Timeout: time.Hour + 42*time.Minute,
		},
		{
			Name:    "FloatTimeout",
			JSON:    `{ timeout: 400.50 }`,
			Timeout: 400*time.Millisecond + 500*time.Microsecond,
		},
		{
			Name:    "IntegerTimeout",
			JSON:    `{ timeout: 2000 }`,
			Timeout: 2000 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime, params := newParamsTestRuntime(t, tc.JSON)

			p, err := newCallParams(testRuntime.VU, params)
			require.NoError(t, err)

			assert.Equal(t, tc.Timeout, p.Timeout)
		})
	}
}

func TestCallParamsDiscardResponseMessageParse(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name                   string
		JSON                   string
		DiscardResponseMessage bool
	}{
		{
			Name:                   "Empty",
			JSON:                   `{}`,
			DiscardResponseMessage: false,
		},
		{
			Name:                   "DiscardResponseMessageFalse",
			JSON:                   `{ discardResponseMessage: false }`,
			DiscardResponseMessage: false,
		},
		{
			Name:                   "DiscardResponseMessageTrue",
			JSON:                   `{ discardResponseMessage: true }`,
			DiscardResponseMessage: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			testRuntime, params := newParamsTestRuntime(t, tc.JSON)

			p, err := newCallParams(testRuntime.VU, params)
			require.NoError(t, err)

			assert.Equal(t, tc.DiscardResponseMessage, p.DiscardResponseMessage)
		})
	}
}

// newParamsTestRuntime creates a new test runtime
// that could be used to test the params
// it also moves to the VU context and creates the params
// Sobek value that could be used in the tests
func newParamsTestRuntime(t *testing.T, paramsJSON string) (*modulestest.Runtime, sobek.Value) {
	t.Helper()

	testRuntime := modulestest.NewRuntime(t)
	registry := metrics.NewRegistry()

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = io.Discard

	state := &lib.State{
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagName,
				metrics.TagURL,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         logger,
	}

	testRuntime.MoveToVUContext(state)

	_, err := testRuntime.VU.Runtime().RunString(`let params = ` + paramsJSON + `;`)
	require.NoError(t, err)

	params := testRuntime.VU.Runtime().Get("params")
	require.False(t, common.IsNullish(params))

	return testRuntime, params
}
