package tests

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElementHandleQueryAll(t *testing.T) {
	const (
		wantLiLen = 2
		query     = "li.ali"
	)

	p := newTestBrowser(t).NewPage(nil)
	p.SetContent(`
		<ul id="aul">
			<li class="ali">1</li>
			<li class="ali">2</li>
		</ul>
  	`, nil)

	t.Run("element_handle", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.Query("#aul").QueryAll(query)))
	})
	t.Run("page", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.QueryAll(query)))
	})
	t.Run("frame", func(t *testing.T) {
		assert.Equal(t, wantLiLen, len(p.MainFrame().QueryAll(query)))
	})
}
