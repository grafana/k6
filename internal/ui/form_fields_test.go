package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringField(t *testing.T) {
	t.Parallel()
	t.Run("Creation", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		assert.Equal(t, "key", f.GetKey())
		assert.Equal(t, "label", f.GetLabel())
	})

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		v, err := f.Clean("uwu")
		assert.NoError(t, err)
		assert.Equal(t, "uwu", v)
	})
	t.Run("Whitespace", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		v, err := f.Clean("\r\n\t ")
		assert.NoError(t, err)
		assert.Equal(t, "", v)
	})
	t.Run("Min", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		f.Min = 10
		_, err := f.Clean("short")
		assert.EqualError(t, err, "invalid input, min length is 10")
	})
	t.Run("Max", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		f.Max = 10
		_, err := f.Clean("too dang long")
		assert.EqualError(t, err, "invalid input, max length is 10")
	})
	t.Run("Default", func(t *testing.T) {
		t.Parallel()
		f := StringField{Key: "key", Label: "label"}
		f.Default = "default"
		v, err := f.Clean("")
		assert.NoError(t, err)
		assert.Equal(t, "default", v)
	})
}
