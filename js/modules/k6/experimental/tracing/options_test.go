package tracing

import "testing"

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	// Note that we prefer variables over constants here
	// as we need to be able to address them.
	var (
		validSampling            = 10
		lowerBoundSampling       = 0
		upperBoundSampling       = 100
		lowerOutOfBoundsSampling = -1
		upperOutOfBoundsSampling = 101
	)

	type fields struct {
		Propagator string
		Sampling   *int
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
			name: "sampling rate is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   &validSampling,
			},
			wantErr: false,
		},
		{
			name: "no sampling is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   nil,
			},
			wantErr: false,
		},
		{
			name: "sampling rate = 0 is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   &lowerBoundSampling,
			},
			wantErr: false,
		},
		{
			name: "sampling rate = 100 is valid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   &upperBoundSampling,
			},
			wantErr: false,
		},
		{
			name: "sampling rate < 0 is invalid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   &lowerOutOfBoundsSampling,
			},
			wantErr: true,
		},
		{
			name: "sampling rater > 100 is invalid",
			fields: fields{
				Propagator: "w3c",
				Sampling:   &upperOutOfBoundsSampling,
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
