package tracing

import (
	"testing"
	"time"
)

func TestTraceID_isValid(t *testing.T) {
	t.Parallel()

	type fields struct {
		Prefix int16
		Code   int8
		Time   time.Time
	}
	testCases := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name: "traceID with k6 cloud code is valid",
			fields: fields{
				Prefix: k6Prefix,
				Code:   k6CloudCode,
				Time:   time.Unix(123456789, 0),
			},
			want: true,
		},
		{
			name: "traceID with k6 local code is valid",
			fields: fields{
				Prefix: k6Prefix,
				Code:   k6LocalCode,
				Time:   time.Unix(123456789, 0),
			},
			want: true,
		},
		{
			name: "traceID with prefix != k6Prefix is invalid",
			fields: fields{
				Prefix: 0,
				Code:   k6CloudCode,
				Time:   time.Unix(123456789, 0),
			},
			want: false,
		},
		{
			name: "traceID code with code != k6CloudCode and code != k6LocalCode is invalid",
			fields: fields{
				Prefix: k6Prefix,
				Code:   0,
				Time:   time.Unix(123456789, 0),
			},
			want: false,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tr := &TraceID{
				Prefix: tc.fields.Prefix,
				Code:   tc.fields.Code,
				Time:   tc.fields.Time,
			}

			if got := tr.isValid(); got != tc.want {
				t.Errorf("TraceID.isValid() = %v, want %v", got, tc.want)
			}
		})
	}
}
