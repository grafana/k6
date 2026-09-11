package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseFrameBaseOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameBaseOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameBaseOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want:  &common.FrameBaseOptions{Strict: true, Timeout: 5 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts := common.NewFrameBaseOptions(defaultTimeout)
			err = parseFrameBaseOptions(opts, vu.Runtime(), v)
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

func TestParseFrameCheckOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameCheckOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameCheckOptions(defaultTimeout),
		},
		{
			name:  "inherited_and_own_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, strict: true})`,
			want: func() *common.FrameCheckOptions {
				o := common.NewFrameCheckOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_position",
			input:   `({position: "invalid"})`,
			wantErr: "could not convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameCheckOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameClickOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameClickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameClickOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({button: "right", clickCount: 2, delay: 50, modifiers: ["Shift"]})`,
			want: func() *common.FrameClickOptions {
				o := common.NewFrameClickOptions(defaultTimeout)
				o.Button, o.ClickCount, o.Delay, o.Modifiers = "right", 2, 50, []string{"Shift"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields_and_strict",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, strict: true})`,
			want: func() *common.FrameClickOptions {
				o := common.NewFrameClickOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_modifiers",
			input:   `({modifiers: "not-an-array"})`,
			wantErr: "parsing element handle click option modifiers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameClickOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameDblclickOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameDblclickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameDblClickOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({button: "middle", delay: 50, modifiers: ["Alt"]})`,
			want: func() *common.FrameDblclickOptions {
				o := common.NewFrameDblClickOptions(defaultTimeout)
				o.Button, o.Delay, o.Modifiers = "middle", 50, []string{"Alt"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields_and_strict",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, strict: true})`,
			want: func() *common.FrameDblclickOptions {
				o := common.NewFrameDblClickOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_modifiers",
			input:   `({modifiers: 123})`,
			wantErr: "could not convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameDblclickOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameFillOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameFillOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameFillOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({force: true, noWaitAfter: true, timeout: 5000, strict: true})`,
			want: func() *common.FrameFillOptions {
				o := common.NewFrameFillOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Strict = true
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameFillOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameGotoOptions(t *testing.T) {
	t.Parallel()

	const (
		defaultReferrer = "https://referrer.example.com/"
		defaultTimeout  = 30 * time.Second
	)

	tests := []struct {
		name    string
		input   string
		want    *common.FrameGotoOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameGotoOptions(defaultReferrer, defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({referer: "https://example.com/", timeout: 1000, waitUntil: "networkidle"})`,
			want: &common.FrameGotoOptions{
				Referer:   "https://example.com/",
				Timeout:   time.Second,
				WaitUntil: common.LifecycleEventNetworkIdle,
			},
		},
		{
			name:    "invalid_waitUntil",
			input:   `({waitUntil: "none"})`,
			wantErr: "parsing goto options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameGotoOptions(vu.Runtime(), v, defaultReferrer, defaultTimeout)
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

func TestParseFrameHoverOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameHoverOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameHoverOptions(defaultTimeout),
		},
		{
			name:  "inherited_and_own_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, modifiers: ["Control"], strict: true})`,
			want: func() *common.FrameHoverOptions {
				o := common.NewFrameHoverOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Modifiers = []string{"Control"}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_modifiers",
			input:   `({modifiers: "not-an-array"})`,
			wantErr: "could not convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameHoverOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameInnerHTMLOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameInnerHTMLOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameInnerHTMLOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameInnerHTMLOptions {
				o := common.NewFrameInnerHTMLOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameInnerHTMLOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameInnerTextOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameInnerTextOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameInnerTextOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameInnerTextOptions {
				o := common.NewFrameInnerTextOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameInnerTextOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameInputValueOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameInputValueOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameInputValueOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameInputValueOptions {
				o := common.NewFrameInputValueOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameInputValueOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsCheckedOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsCheckedOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsCheckedOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameIsCheckedOptions {
				o := common.NewFrameIsCheckedOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsCheckedOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsDisabledOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsDisabledOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsDisabledOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameIsDisabledOptions {
				o := common.NewFrameIsDisabledOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsDisabledOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsEditableOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsEditableOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsEditableOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameIsEditableOptions {
				o := common.NewFrameIsEditableOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsEditableOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsEnabledOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsEnabledOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsEnabledOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameIsEnabledOptions {
				o := common.NewFrameIsEnabledOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsEnabledOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsInViewportOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameIsInViewportOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsInViewportOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000, ratio: 0.5})`,
			want: func() *common.FrameIsInViewportOptions {
				o := common.NewFrameIsInViewportOptions(defaultTimeout)
				o.Strict, o.Timeout, o.Ratio = true, 5*time.Second, 0.5
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsInViewportOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameIsHiddenOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsHiddenOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsHiddenOptions(),
		},
		{
			name:  "strict",
			input: `({strict: true})`,
			want:  &common.FrameIsHiddenOptions{Strict: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsHiddenOptions(vu.Runtime(), v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameIsVisibleOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  *common.FrameIsVisibleOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameIsVisibleOptions(),
		},
		{
			name:  "strict",
			input: `({strict: true})`,
			want:  &common.FrameIsVisibleOptions{Strict: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameIsVisibleOptions(vu.Runtime(), v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFramePressOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FramePressOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFramePressOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({delay: 50, noWaitAfter: true, timeout: 5000})`,
			want: func() *common.FramePressOptions {
				o := common.NewFramePressOptions(defaultTimeout)
				o.Delay, o.NoWaitAfter, o.Timeout = 50, true, 5*time.Second
				return o
			}(),
		},
		{
			// FramePressOptions never parsed `strict` even before this migration
			// (no Parse override existed; it relied on promotion from
			// ElementHandlePressOptions, which has no Strict field at all).
			// This is intentionally preserved dormant behaviour, not a bug.
			name:  "strict_field_is_dormant",
			input: `({strict: true})`,
			want:  common.NewFramePressOptions(defaultTimeout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFramePressOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameSelectOptionOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameSelectOptionOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameSelectOptionOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({force: true, noWaitAfter: true, timeout: 5000, strict: true})`,
			want: func() *common.FrameSelectOptionOptions {
				o := common.NewFrameSelectOptionOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Strict = true
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameSelectOptionOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameSetContentOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameSetContentOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameSetContentOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({timeout: 1000, waitUntil: "networkidle"})`,
			want: &common.FrameSetContentOptions{
				Timeout:   time.Second,
				WaitUntil: common.LifecycleEventNetworkIdle,
			},
		},
		{
			name:    "invalid_waitUntil",
			input:   `({waitUntil: "none"})`,
			wantErr: "parsing setContent options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameSetContentOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameSetInputFilesOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameSetInputFilesOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameSetInputFilesOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000})`,
			want: func() *common.FrameSetInputFilesOptions {
				o := common.NewFrameSetInputFilesOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				return o
			}(),
		},
		{
			// FrameSetInputFilesOptions never parsed `strict` even before this
			// migration (pure delegation to ElementHandleSetInputFilesOptions.Parse,
			// which has no Strict field). Intentionally preserved dormant behaviour.
			name:  "strict_field_is_dormant",
			input: `({strict: true})`,
			want:  common.NewFrameSetInputFilesOptions(defaultTimeout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameSetInputFilesOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameTapOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameTapOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameTapOptions(defaultTimeout),
		},
		{
			name:  "inherited_and_own_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, modifiers: ["Meta"], strict: true})`,
			want: func() *common.FrameTapOptions {
				o := common.NewFrameTapOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Modifiers = []string{"Meta"}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_modifiers",
			input:   `({modifiers: "not-an-array"})`,
			wantErr: "could not convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameTapOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameTextContentOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameTextContentOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameTextContentOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameTextContentOptions {
				o := common.NewFrameTextContentOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameTextContentOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameTypeOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameTypeOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameTypeOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({delay: 50, noWaitAfter: true, timeout: 5000})`,
			want: func() *common.FrameTypeOptions {
				o := common.NewFrameTypeOptions(defaultTimeout)
				o.Delay, o.NoWaitAfter, o.Timeout = 50, true, 5*time.Second
				return o
			}(),
		},
		{
			// FrameTypeOptions never parsed `strict` even before this migration
			// (no Parse override existed; it relied on promotion from
			// ElementHandleTypeOptions, which has no Strict field at all).
			// This is intentionally preserved dormant behaviour, not a bug.
			name:  "strict_field_is_dormant",
			input: `({strict: true})`,
			want:  common.NewFrameTypeOptions(defaultTimeout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameTypeOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameUncheckOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameUncheckOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameUncheckOptions(defaultTimeout),
		},
		{
			name:  "inherited_base_pointer_fields_and_strict",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}, strict: true})`,
			want: func() *common.FrameUncheckOptions {
				o := common.NewFrameUncheckOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
				o.Strict = true
				return o
			}(),
		},
		{
			name:    "invalid_position",
			input:   `({position: "invalid"})`,
			wantErr: "could not convert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameUncheckOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameWaitForLoadStateOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameWaitForLoadStateOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameWaitForLoadStateOptions(defaultTimeout),
		},
		{
			name:  "timeout",
			input: `({timeout: 5000})`,
			want:  &common.FrameWaitForLoadStateOptions{Timeout: 5 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameWaitForLoadStateOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameWaitForSelectorOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameWaitForSelectorOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameWaitForSelectorOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({state: "hidden", strict: true, timeout: 5000})`,
			want: &common.FrameWaitForSelectorOptions{
				State:   common.DOMElementStateHidden,
				Strict:  true,
				Timeout: 5 * time.Second,
			},
		},
		{
			name:    "invalid_state",
			input:   `({state: "bogus"})`,
			wantErr: `"bogus" is not a valid DOM state`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameWaitForSelectorOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameDispatchEventOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.FrameDispatchEventOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameDispatchEventOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({strict: true, timeout: 5000})`,
			want: func() *common.FrameDispatchEventOptions {
				o := common.NewFrameDispatchEventOptions(defaultTimeout)
				o.Strict, o.Timeout = true, 5*time.Second
				return o
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameDispatchEventOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseFrameWaitForFunctionOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameWaitForFunctionOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameWaitForFunctionOptions(defaultTimeout),
		},
		{
			name:  "polling_string",
			input: `({timeout: 5000, polling: "mutation"})`,
			want: &common.FrameWaitForFunctionOptions{
				Polling: common.PollingMutation,
				Timeout: 5 * time.Second,
			},
		},
		{
			name:  "polling_number",
			input: `({polling: 100})`,
			want: &common.FrameWaitForFunctionOptions{
				Polling:  common.PollingInterval,
				Interval: 100,
				Timeout:  defaultTimeout,
			},
		},
		{
			name:    "invalid_polling",
			input:   `({polling: "bogus"})`,
			wantErr: "wrong polling option value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameWaitForFunctionOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameWaitForURLOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameWaitForURLOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameWaitForURLOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({timeout: 1000, waitUntil: "networkidle"})`,
			want: &common.FrameWaitForURLOptions{
				Timeout:   time.Second,
				WaitUntil: common.LifecycleEventNetworkIdle,
			},
		},
		{
			name:    "invalid_waitUntil",
			input:   `({waitUntil: "none"})`,
			wantErr: "parsing waitForURL options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameWaitForURLOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseFrameWaitForNavigationOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.FrameWaitForNavigationOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewFrameWaitForNavigationOptions(defaultTimeout),
		},
		{
			name:  "string_url_is_quoted",
			input: `({url: "https://example.com/", timeout: 1000, waitUntil: "networkidle"})`,
			want: &common.FrameWaitForNavigationOptions{
				URL:       `'https://example.com/'`,
				Timeout:   time.Second,
				WaitUntil: common.LifecycleEventNetworkIdle,
			},
		},
		{
			name:    "invalid_waitUntil",
			input:   `({waitUntil: "none"})`,
			wantErr: "parsing waitForNavigation options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseFrameWaitForNavigationOptions(vu.Runtime(), v, defaultTimeout)
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
