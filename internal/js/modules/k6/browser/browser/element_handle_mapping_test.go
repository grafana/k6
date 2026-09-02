package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestParseElementHandleBaseOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleBaseOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleBaseOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({force: true, noWaitAfter: true, timeout: 5000})`,
			want: &common.ElementHandleBaseOptions{
				Force:       true,
				NoWaitAfter: true,
				Timeout:     5 * time.Second,
			},
		},
		{
			name:  "partial_option",
			input: `({timeout: 1000})`,
			want: &common.ElementHandleBaseOptions{
				Force:       false,
				NoWaitAfter: false,
				Timeout:     1 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts := common.NewElementHandleBaseOptions(defaultTimeout)
			err = parseElementHandleBaseOptions(opts, vu.Runtime(), v)
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

func TestParseElementHandleBasePointerOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleBasePointerOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleBasePointerOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1.5, y: 2.5}})`,
			want: func() *common.ElementHandleBasePointerOptions {
				o := common.NewElementHandleBasePointerOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1.5, Y: 2.5}
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

			opts := common.NewElementHandleBasePointerOptions(defaultTimeout)
			err = parseElementHandleBasePointerOptions(opts, vu.Runtime(), v)
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

func TestParseElementHandleClickOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleClickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleClickOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({button: "right", clickCount: 3, delay: 100, modifiers: ["Shift", "Control"]})`,
			want: func() *common.ElementHandleClickOptions {
				o := common.NewElementHandleClickOptions(defaultTimeout)
				o.Button, o.ClickCount, o.Delay = "right", 3, 100
				o.Modifiers = []string{"Shift", "Control"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}})`,
			want: func() *common.ElementHandleClickOptions {
				o := common.NewElementHandleClickOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
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

			opts, err := parseElementHandleClickOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseElementHandleDblclickOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleDblclickOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleDblclickOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({button: "middle", delay: 50, modifiers: ["Alt"]})`,
			want: func() *common.ElementHandleDblclickOptions {
				o := common.NewElementHandleDblclickOptions(defaultTimeout)
				o.Button, o.Delay, o.Modifiers = "middle", 50, []string{"Alt"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}})`,
			want: func() *common.ElementHandleDblclickOptions {
				o := common.NewElementHandleDblclickOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
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

			opts, err := parseElementHandleDblclickOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseElementHandleHoverOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleHoverOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleHoverOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({modifiers: ["Shift"]})`,
			want: func() *common.ElementHandleHoverOptions {
				o := common.NewElementHandleHoverOptions(defaultTimeout)
				o.Modifiers = []string{"Shift"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}})`,
			want: func() *common.ElementHandleHoverOptions {
				o := common.NewElementHandleHoverOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
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

			opts, err := parseElementHandleHoverOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseElementHandleTapOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleTapOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleTapOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({modifiers: ["Control"]})`,
			want: func() *common.ElementHandleTapOptions {
				o := common.NewElementHandleTapOptions(defaultTimeout)
				o.Modifiers = []string{"Control"}
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}})`,
			want: func() *common.ElementHandleTapOptions {
				o := common.NewElementHandleTapOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
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

			opts, err := parseElementHandleTapOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseElementHandleSetCheckedOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name    string
		input   string
		want    *common.ElementHandleSetCheckedOptions
		wantErr string
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleSetCheckedOptions(defaultTimeout),
		},
		{
			name:  "own_fields",
			input: `({strict: true})`,
			want: func() *common.ElementHandleSetCheckedOptions {
				o := common.NewElementHandleSetCheckedOptions(defaultTimeout)
				o.Strict = true
				return o
			}(),
		},
		{
			name:  "inherited_base_pointer_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000, trial: true, position: {x: 1, y: 2}})`,
			want: func() *common.ElementHandleSetCheckedOptions {
				o := common.NewElementHandleSetCheckedOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
				o.Trial, o.Position = true, &common.Position{X: 1, Y: 2}
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

			opts, err := parseElementHandleSetCheckedOptions(vu.Runtime(), v, defaultTimeout)
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

func TestParseElementHandleSetInputFilesOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.ElementHandleSetInputFilesOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleSetInputFilesOptions(defaultTimeout),
		},
		{
			name:  "inherited_base_fields",
			input: `({force: true, noWaitAfter: true, timeout: 5000})`,
			want: func() *common.ElementHandleSetInputFilesOptions {
				o := common.NewElementHandleSetInputFilesOptions(defaultTimeout)
				o.Force, o.NoWaitAfter, o.Timeout = true, true, 5*time.Second
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

			opts, err := parseElementHandleSetInputFilesOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseElementHandlePressOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.ElementHandlePressOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandlePressOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({delay: 100, noWaitAfter: true, timeout: 2000})`,
			want: &common.ElementHandlePressOptions{
				Delay:       100,
				NoWaitAfter: true,
				Timeout:     2 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseElementHandlePressOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseElementHandleTypeOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.ElementHandleTypeOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleTypeOptions(defaultTimeout),
		},
		{
			name:  "all_options",
			input: `({delay: 50, noWaitAfter: true, timeout: 3000})`,
			want: &common.ElementHandleTypeOptions{
				Delay:       50,
				NoWaitAfter: true,
				Timeout:     3 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseElementHandleTypeOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseElementHandleWaitForElementStateOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.ElementHandleWaitForElementStateOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleWaitForElementStateOptions(defaultTimeout),
		},
		{
			name:  "timeout_option",
			input: `({timeout: 1500})`,
			want:  &common.ElementHandleWaitForElementStateOptions{Timeout: 1500 * time.Millisecond},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseElementHandleWaitForElementStateOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}

func TestParseElementHandleScreenshotOptions(t *testing.T) {
	t.Parallel()

	const defaultTimeout = 30 * time.Second

	tests := []struct {
		name  string
		input string
		want  *common.ElementHandleScreenshotOptions
	}{
		{
			name:  "defaults_on_null",
			input: `null`,
			want:  common.NewElementHandleScreenshotOptions(defaultTimeout),
		},
		{
			name:  "explicit_type_overrides_path_inference",
			input: `({omitBackground: true, path: "img.png", quality: 80, type: "jpeg", timeout: 2000})`,
			want: &common.ElementHandleScreenshotOptions{
				OmitBackground: true,
				Path:           "img.png",
				Quality:        80,
				Format:         common.ImageFormatJPEG,
				Timeout:        2 * time.Second,
			},
		},
		{
			name:  "format_inferred_by_jpg_path",
			input: `({path: "photo.jpg"})`,
			want: func() *common.ElementHandleScreenshotOptions {
				o := common.NewElementHandleScreenshotOptions(defaultTimeout)
				o.Path, o.Format = "photo.jpg", common.ImageFormatJPEG
				return o
			}(),
		},
		{
			name:  "format_inferred_by_jpeg_path",
			input: `({path: "photo.jpeg"})`,
			want: func() *common.ElementHandleScreenshotOptions {
				o := common.NewElementHandleScreenshotOptions(defaultTimeout)
				o.Path, o.Format = "photo.jpeg", common.ImageFormatJPEG
				return o
			}(),
		},
		{
			name:  "png_path_stays_default_format",
			input: `({path: "photo.png"})`,
			want: func() *common.ElementHandleScreenshotOptions {
				o := common.NewElementHandleScreenshotOptions(defaultTimeout)
				o.Path = "photo.png"
				return o
			}(),
		},
		{
			name:  "unknown_type_ignored",
			input: `({type: "gif"})`,
			want:  common.NewElementHandleScreenshotOptions(defaultTimeout),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			vu := k6test.NewVU(t)
			v, err := vu.Runtime().RunString(tt.input)
			require.NoError(t, err)

			opts, err := parseElementHandleScreenshotOptions(vu.Runtime(), v, defaultTimeout)
			require.NoError(t, err)
			assert.Equal(t, tt.want, opts)
		})
	}
}
