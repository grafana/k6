package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"
)

func TestMouseDblClick(t *testing.T) {
	t.Parallel()

	b := newTestBrowser(t, withFileServer())
	p := b.NewPage(nil)
	opts := &common.FrameGotoOptions{
		Timeout: common.DefaultTimeout,
	}
	_, err := p.Goto(
		b.staticURL("dbl_click.html"),
		opts,
	)
	require.NoError(t, err)

	p.Mouse.DblClick(35, 17, nil)

	v := p.Evaluate(b.toGojaValue(`() => window.dblclick`))
	assert.True(t, b.asGojaBool(v), "failed to double click the link")

	got := p.InnerText("#counter", nil)
	assert.Equal(t, "2", got)
}
