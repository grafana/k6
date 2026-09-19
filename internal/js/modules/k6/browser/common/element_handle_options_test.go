package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
)

func TestElementHandleBasePointerOptionsParse(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		opts      map[string]any
		expectErr bool
		verify    func(t *testing.T, opts *ElementHandleBasePointerOptions)
	}{
		"valid_full": {
			opts: map[string]any{
				"timeout": 1234,
				"force":   true,
				"trial":   true,
				"position": map[string]any{
					"x": 10,
					"y": 20,
				},
			},
			verify: func(t *testing.T, opts *ElementHandleBasePointerOptions) {
				assert.Equal(t, 1234*time.Millisecond, opts.Timeout)
				assert.True(t, opts.Force)
				assert.True(t, opts.Trial)
				require.NotNil(t, opts.Position)
				assert.Equal(t, 10.0, opts.Position.X)
				assert.Equal(t, 20.0, opts.Position.Y)
			},
		},
		"valid_minimal": {
			opts: map[string]any{},
			verify: func(t *testing.T, opts *ElementHandleBasePointerOptions) {
				assert.False(t, opts.Trial)
				assert.Nil(t, opts.Position)
				// Default timeout passed to NewElementHandleBasePointerOptions is 500ms in the test
				assert.Equal(t, 500*time.Millisecond, opts.Timeout)
			},
		},
		"null_options": {
			opts: nil,
			verify: func(t *testing.T, opts *ElementHandleBasePointerOptions) {
				assert.Nil(t, opts.Position)
				assert.Equal(t, 500*time.Millisecond, opts.Timeout)
			},
		},
		"invalid_position_type": {
			opts: map[string]any{
				"position": "invalid",
			},
			expectErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			vu := k6test.NewVU(t)
			ctx := vu.Context()
			rt := vu.Runtime()

			val := rt.ToValue(test.opts)

			opts := NewElementHandleBasePointerOptions(500 * time.Millisecond)
			err := opts.Parse(ctx, val)

			if test.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if test.verify != nil {
					test.verify(t, opts)
				}
			}
		})
	}
}

func TestElementHandleClickOptionsToMouseClickOptions(t *testing.T) {
	t.Parallel()

	src := &ElementHandleClickOptions{
		Button:     "right",
		ClickCount: 2,
		Delay:      50,
		Modifiers:  []string{"Shift", "ControlOrMeta"},
	}

	got := src.ToMouseClickOptions()

	assert.Equal(t, "right", got.Button)
	assert.Equal(t, int64(2), got.ClickCount)
	assert.Equal(t, int64(50), got.Delay)
	assert.Equal(t, []string{"Shift", "ControlOrMeta"}, got.Modifiers)
}

func TestElementHandleDblclickOptionsToMouseClickOptions(t *testing.T) {
	t.Parallel()

	src := &ElementHandleDblclickOptions{
		Button:    "right",
		Delay:     50,
		Modifiers: []string{"Alt"},
	}

	got := src.ToMouseClickOptions()

	assert.Equal(t, "right", got.Button)
	assert.Equal(t, int64(2), got.ClickCount)
	assert.Equal(t, int64(50), got.Delay)
	assert.Equal(t, []string{"Alt"}, got.Modifiers)
}
