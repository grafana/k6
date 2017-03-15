package compiler

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equal(t, `"use strict";(function () {return true;});`, src)
		assert.Equal(t, 3, srcmap.Version)
		assert.Equal(t, "test.js", srcmap.File)
		assert.Equal(t, "aAAA,qBAAK,IAAL", srcmap.Mappings)
	})
	t.Run("longer", func(t *testing.T) {
		src, srcmap, err := c.Transform(strings.Join([]string{
			`function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), "test.js")
		println(src)
		assert.NoError(t, err)
		assert.Equal(t, strings.Join([]string{
			`"use strict";function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`var res = add(1, 2);`,
		}, "\n"), src)
		assert.Equal(t, 3, srcmap.Version)
		assert.Equal(t, "test.js", srcmap.File)
		assert.Equal(t, "aAAA,SAASA,GAAT,CAAaC,CAAb,EAAgBC,CAAhB,EAAmB;AACf,WAAOD,IAAIC,CAAX;AACH;;AAED,IAAIC,MAAMH,IAAI,CAAJ,EAAO,CAAP,CAAV", srcmap.Mappings)
	})
}
