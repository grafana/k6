package common

import (
	"testing"

	"github.com/grafana/xk6-browser/k6/k6test"

	"github.com/stretchr/testify/assert"
)

func TestBrowserContextOptionsPermissions(t *testing.T) {
	vu := k6test.NewVU(t)

	var opts BrowserContextOptions
	err := opts.Parse(vu.Context(), vu.ToGojaValue((struct {
		Permissions []interface{} `js:"permissions"`
	}{
		Permissions: []interface{}{"camera", "microphone"},
	})))
	assert.NoError(t, err)
	assert.Len(t, opts.Permissions, 2)
	assert.Equal(t, opts.Permissions, []string{"camera", "microphone"})
}
