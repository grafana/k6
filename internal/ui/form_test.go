package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForm(t *testing.T) {
	t.Parallel()
	t.Run("Blank", func(t *testing.T) {
		t.Parallel()
		data, err := Form{}.Run(strings.NewReader(""), bytes.NewBuffer(nil))
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{}, data)
	})
	t.Run("Banner", func(t *testing.T) {
		t.Parallel()
		out := bytes.NewBuffer(nil)
		data, err := Form{Banner: "Hi!"}.Run(strings.NewReader(""), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{}, data)
		assert.Equal(t, "Hi!\n\n", out.String())
	})
	t.Run("Field", func(t *testing.T) {
		t.Parallel()
		f := Form{
			Fields: []Field{
				StringField{Key: "key", Label: "label"},
			},
		}
		in := "Value\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "Value"}, data)
		assert.Equal(t, "  label: ", out.String())
	})
	t.Run("Fields", func(t *testing.T) {
		t.Parallel()
		f := Form{
			Fields: []Field{
				StringField{Key: "a", Label: "label a"},
				StringField{Key: "b", Label: "label b"},
			},
		}
		in := "1\n2\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "1", "b": "2"}, data)
		assert.Equal(t, "  label a:   label b: ", out.String())
	})
	t.Run("Defaults", func(t *testing.T) {
		t.Parallel()
		f := Form{
			Fields: []Field{
				StringField{Key: "a", Label: "label a", Default: "default a"},
				StringField{Key: "b", Label: "label b", Default: "default b"},
			},
		}
		in := "\n2\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"a": "default a", "b": "2"}, data)
		assert.Equal(t, "  label a [default a]:   label b [default b]: ", out.String())
	})
	t.Run("Errors", func(t *testing.T) {
		t.Parallel()
		f := Form{
			Fields: []Field{
				StringField{Key: "key", Label: "label", Min: 6, Max: 10},
			},
		}
		in := "short\ntoo damn long\nperfect\n"
		out := bytes.NewBuffer(nil)
		data, err := f.Run(strings.NewReader(in), out)
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"key": "perfect"}, data)
		assert.Equal(t, "  label: - invalid input, min length is 6\n  label: - invalid input, max length is 10\n  label: ", out.String())
	})
}
