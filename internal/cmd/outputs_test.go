package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestBuiltinOutputString(t *testing.T) {
	t.Parallel()
	exp := []string{
		"cloud", "csv", "datadog", "experimental-prometheus-rw",
		"influxdb", "json", "kafka", "statsd", "experimental-opentelemetry",
		"summary",
	}
	assert.Equal(t, exp, builtinOutputStrings())
}

func TestDeriveOutputs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  Config
		want []string
	}{
		{
			name: "empty",
			cfg:  Config{},
			want: nil,
		},
		{
			name: "single",
			cfg:  Config{Out: []string{"json"}},
			want: []string{"json"},
		},
		{
			name: "multiple",
			cfg:  Config{Out: []string{"json", "cloud", "csv"}},
			want: []string{"json", "cloud", "csv"},
		},
		{
			name: "duplicate",
			cfg:  Config{Out: []string{"json", "json"}},
			want: []string{"json"},
		},
		// web dashboard
		{
			name: "web-dashboard",
			cfg: Config{
				WebDashboard: null.BoolFrom(true),
				Out:          []string{"json"},
			},
			want: []string{"json", "web-dashboard"},
		},
		// no web-dashboard
		{
			name: "no-web-dashboard",
			cfg: Config{
				WebDashboard: null.BoolFrom(false),
				Out:          []string{"json"},
			},
			want: []string{"json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := deriveOutputs(tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}
