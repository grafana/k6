package compiler

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	c, err := New()
	assert.NotNil(t, c)
	assert.NoError(t, err)
}

func TestTransform(t *testing.T) {
	c, err := New()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("blank", func(t *testing.T) {
		src, srcmap, err := c.Transform("", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";`, src)
		assert.Equal(t, 3, srcmap.Version)
		assert.Equal(t, "test.js", srcmap.File)
		assert.Equal(t, "", srcmap.Mappings)
	})
	t.Run("double-arrow", func(t *testing.T) {
		src, srcmap, err := c.Transform("()=> true", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";(function(){return true;});`, src)
		assert.Equal(t, 3, srcmap.Version)
		assert.Equal(t, "test.js", srcmap.File)
		assert.Equal(t, "aAAA,kBAAK,KAAL", srcmap.Mappings)
	})
}
