package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseMouseClickOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *common.MouseClickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  &common.MouseClickOptions{Button: "left", ClickCount: 1, Delay: 0},
		},
		{
			name:  "all_options",
			input: `({button: "right", clickCount: 3, delay: 100})`,
			want:  &common.MouseClickOptions{Button: "right", ClickCount: 3, Delay: 100},
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
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseMouseDblClickOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *common.MouseDblClickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  &common.MouseDblClickOptions{Button: "left", Delay: 0},
		},
		{
			name:  "all_options",
			input: `({button: "right", delay: 50})`,
			want:  &common.MouseDblClickOptions{Button: "right", Delay: 50},
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
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseMouseDownUpOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *common.MouseDownUpOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  &common.MouseDownUpOptions{Button: "left", ClickCount: 1},
		},
		{
			name:  "all_options",
			input: `({button: "middle", clickCount: 2})`,
			want:  &common.MouseDownUpOptions{Button: "middle", ClickCount: 2},
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
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseMouseMoveOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *common.MouseMoveOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  &common.MouseMoveOptions{Steps: 1},
		},
		{
			name:  "custom_steps",
			input: `({steps: 10})`,
			want:  &common.MouseMoveOptions{Steps: 10},
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
			assert.Equal(t, tt.want, opts)
		})
	}
}
