package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserContextOptionsPermissions(t *testing.T) {
	vu := newMockVU(t)

	var opts BrowserContextOptions
	err := opts.Parse(vu.CtxField, vu.RuntimeField.ToValue((struct {
		Permissions []interface{} `js:"permissions"`
	}{
		Permissions: []interface{}{"camera", "microphone"},
	})))
	assert.NoError(t, err)
	assert.Len(t, opts.Permissions, 2)
	assert.Equal(t, opts.Permissions, []string{"camera", "microphone"})
}
