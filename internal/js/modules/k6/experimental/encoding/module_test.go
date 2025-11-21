package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleTextDecoder(t *testing.T) {
	t.Parallel()

	t.Run("TextDecoder.Decode", func(t *testing.T) {
		t.Parallel()

		ts := newTestSetup(t)

		_, err := ts.rt.RunOnEventLoop(`
			new TextDecoder().decode(undefined)
		`)

		assert.NoError(t, err)
	})
}
