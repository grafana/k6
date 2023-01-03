package tracing

import "testing"

func TestTraceID_isValid(t *testing.T) {
	t.Parallel()

	type fields struct {
		Prefix            int16
		Code              int8
		UnixTimestampNano uint64
	}
	testCases := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "traceID with k6 cloud code is valid",
			fields: fields{
				Prefix:            k6Prefix,
				Code:              k6CloudCode,
				UnixTimestampNano: 123456789,
			},
			want: true,
		},
		{
			name: "traceID with k6 local code is valid",
			fields: fields{
				Prefix:            k6Prefix,
				Code:              k6LocalCode,
				UnixTimestampNano: 123456789,
			},
			want: true,
		},
		{
			name: "traceID with prefix != k6Prefix is invalid",
			fields: fields{
				Prefix:            0,
				Code:              k6CloudCode,
				UnixTimestampNano: 123456789,
			},
			want: false,
		},
		{
			name: "traceID code with code != k6CloudCode and code != k6LocalCode is invalid",
			fields: fields{
				Prefix:            k6Prefix,
				Code:              0,
				UnixTimestampNano: 123456789,
			},
			want: false,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := &TraceID{
				Prefix:            tc.fields.Prefix,
				Code:              tc.fields.Code,
				UnixTimestampNano: tc.fields.UnixTimestampNano,
			}

			if got := tr.isValid(); got != tc.want {
				t.Errorf("TraceID.isValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
