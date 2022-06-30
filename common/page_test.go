package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPageLocator can be removed later on when we add integration
// tests. Since we don't yet have any of them, it makes sense to keep
// this test.
func TestPageLocator(t *testing.T) {
	const (
		wantMainFrameID = "1"
		wantSelector    = "span"
	)
	ctx := context.TODO()
	p := &Page{
		ctx: ctx,
		frameManager: &FrameManager{
			ctx:       ctx,
			mainFrame: &Frame{id: wantMainFrameID, ctx: ctx},
		},
	}
	v := p.Locator(wantSelector, nil)
	require.IsType(t, v, &Locator{})
	l, _ := v.(*Locator)
	assert.Equal(t, wantSelector, l.selector)
	assert.Equal(t, wantMainFrameID, string(l.frame.id))

	// other behavior will be tested via integration tests
}
