package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseMouseClickOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantButton  string
		wantClick   int64
		wantDelay   int64
		wantErr     string
	}{
		{
			name:       "defaults_on_null",
			input:      `null`,
			wantButton: "left",
			wantClick:  1,
			wantDelay:  0,
		},
		{
			name:       "all_options",
			input:      `({button: "right", clickCount: 3, delay: 100})`,
			wantButton: "right",
			wantClick:  3,
			wantDelay:  100,
		},
		{
			name:    "invalid_clickCount",
			input:   `({clickCount: "invalid"})`,
			wantErr: "clickCount must be an integer",
		},
		{
			name:    "invalid_delay",
			input:   `({delay: true})`,
			wantErr: "delay must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseMouseClickOptions(vu.Runtime(), v)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantButton, opts.Button)
			assert.Equal(t, tt.wantClick, opts.ClickCount)
			assert.Equal(t, tt.wantDelay, opts.Delay)
		})
	}
}

func TestParseMouseDblClickOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantButton string
		wantDelay  int64
		wantErr    string
	}{
		{
			name:       "defaults_on_null",
			input:      `null`,
			wantButton: "left",
			wantDelay:  0,
		},
		{
			name:       "all_options",
			input:      `({button: "right", delay: 50})`,
			wantButton: "right",
			wantDelay:  50,
		},
		{
			name:    "invalid_delay",
			input:   `({delay: "slow"})`,
			wantErr: "delay must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseMouseDblClickOptions(vu.Runtime(), v)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantButton, opts.Button)
			assert.Equal(t, tt.wantDelay, opts.Delay)
		})
	}
}

func TestParseMouseDownUpOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantButton string
		wantClick  int64
		wantErr    string
	}{
		{
			name:       "defaults_on_null",
			input:      `null`,
			wantButton: "left",
			wantClick:  1,
		},
		{
			name:       "all_options",
			input:      `({button: "middle", clickCount: 2})`,
			wantButton: "middle",
			wantClick:  2,
		},
		{
			name:    "invalid_clickCount",
			input:   `({clickCount: []})`,
			wantErr: "clickCount must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseMouseDownUpOptions(vu.Runtime(), v)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantButton, opts.Button)
			assert.Equal(t, tt.wantClick, opts.ClickCount)
		})
	}
}

func TestParseMouseMoveOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantSteps int64
		wantErr   string
	}{
		{
			name:      "defaults_on_null",
			input:     `null`,
			wantSteps: 1,
		},
		{
			name:      "custom_steps",
			input:     `({steps: 10})`,
			wantSteps: 10,
		},
		{
			name:    "invalid_steps",
			input:   `({steps: {}})`,
			wantErr: "steps must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseMouseMoveOptions(vu.Runtime(), v)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantSteps, opts.Steps)
		})
	}
}
