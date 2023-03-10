package tracing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOptionsWithoutSamplingProperty(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		const options = {
			propagator: 'w3c',
		}
	`)
	require.NoError(t, err)
	optionsValue := ts.TestRuntime.VU.Runtime().Get("options")
	gotOptions, gotOptionsErr := newOptions(ts.TestRuntime.VU.Runtime(), optionsValue)

	assert.NoError(t, gotOptionsErr)
	assert.Equal(t, 1.0, gotOptions.Sampling)
}

func TestNewOptionsWithSamplingPropertySetToNullish(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		const options = {
			propagator: 'w3c',
			sampling: null,
		}
	`)
	require.NoError(t, err)
	optionsValue := ts.TestRuntime.VU.Runtime().Get("options")
	gotOptions, gotOptionsErr := newOptions(ts.TestRuntime.VU.Runtime(), optionsValue)

	assert.NoError(t, gotOptionsErr)
	assert.Equal(t, 1.0, gotOptions.Sampling)
}

func TestNewOptionsWithSamplingPropertySet(t *testing.T) {
	t.Parallel()

	ts := newTestSetup(t)

	_, err := ts.TestRuntime.VU.Runtime().RunString(`
		const options = {
			propagator: 'w3c',
			sampling: 0.5,
		}
	`)
	require.NoError(t, err)
	optionsValue := ts.TestRuntime.VU.Runtime().Get("options")
	gotOptions, gotOptionsErr := newOptions(ts.TestRuntime.VU.Runtime(), optionsValue)

	assert.NoError(t, gotOptionsErr)
	assert.Equal(t, 0.5, gotOptions.Sampling)
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	// Note that we prefer variables over constants here
	// as we need to be able to address them.
	var (
		validSampling            = 0.1
		lowerBoundSampling       = 0.0
		upperBoundSampling       = 1.0
		lowerOutOfBoundsSampling = -1.0
		upperOutOfBoundsSampling = 1.01
	)

	type fields struct {
		Propagator string
		Sampling   float64
		Baggage    map[string]string
	}
	testCases := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "w3c propagator is valid",
			fields: fields{
				Propagator: "w3c",
			},
			wantErr: false,
		},
		{
			name: "jaeger propagator is valid",
			fields: fields{
				Propagator: "jaeger",
			},
			wantErr: false,
		},
		{
			name: "invalid propagator is invalid",
			fields: fields{
				Propagator: "invalid",
			},
			wantErr: true,
		},
		{
			name: "sampling is not rate is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   validSampling,
			},
			wantErr: false,
		},
		{
			name: "sampling rate = 0 is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   lowerBoundSampling,
			},
			wantErr: false,
		},
		{
			name: "sampling rate = 1.0 is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   upperBoundSampling,
			},
			wantErr: false,
		},
		{
			name: "sampling rate < 0 is invalid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   lowerOutOfBoundsSampling,
			},
			wantErr: true,
		},
		{
			name: "sampling rater > 1.0 is invalid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   upperOutOfBoundsSampling,
			},
			wantErr: true,
		},
		{
			name: "baggage is not yet supported",
			fields: fields{
				Propagator: "w3c",
				Baggage:    map[string]string{"key": "value"},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			i := &options{
				Propagator: tc.fields.Propagator,
				Sampling:   tc.fields.Sampling,
				Baggage:    tc.fields.Baggage,
			}

			if err := i.validate(); (err != nil) != tc.wantErr {
				t.Errorf("instrumentationOptions.validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
