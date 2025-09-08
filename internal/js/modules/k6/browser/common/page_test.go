package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPageLocator can be removed later on when we add integration
// tests. Since we don't yet have any of them, it makes sense to keep
// this test.
func TestPageLocator(t *testing.T) {
	t.Parallel()

	const (
		wantMainFrameID = "1"
		wantSelector    = "span"
	)
	p := &Page{
		ctx: t.Context(),
		frameManager: &FrameManager{
			ctx:       t.Context(),
			mainFrame: &Frame{id: wantMainFrameID, ctx: t.Context()},
		},
	}
	l := p.Locator(wantSelector, nil)
	assert.Equal(t, wantSelector, l.selector)
	assert.Equal(t, wantMainFrameID, string(l.frame.id))

	// other behavior will be tested via integration tests
}
