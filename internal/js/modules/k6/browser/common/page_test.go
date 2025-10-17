package common

import (
	"context"
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
	ctx := context.TODO()
	p := &Page{
		ctx: ctx,
		frameManager: &FrameManager{
			ctx:       ctx,
			mainFrame: &Frame{id: wantMainFrameID, ctx: ctx},
		},
	}
	l := p.Locator(wantSelector, nil)
	assert.Equal(t, wantSelector, l.selector)
	assert.Equal(t, wantMainFrameID, string(l.frame.id))

	// other behavior will be tested via integration tests
}

func TestPageEventHandlersLifecycle(t *testing.T) {
	t.Parallel()

	p := &Page{
		eventHandlers: make(map[PageOnEventName][]pageOnHandlerRecord),
	}

	// Add some handlers to the page event handling system.
	handler := func(PageOnEvent) error { return nil }
	id1, err := p.addEventHandler(EventPageRequestCalled, handler)
	assert.NoError(t, err)
	id2, err := p.addEventHandler(EventPageRequestCalled, handler)
	assert.NoError(t, err)
	id3, err := p.addEventHandler(EventPageResponseCalled, handler)
	assert.NoError(t, err)

	// Check if the handlers are registered correctly.
	assert.True(t, p.hasPageOnHandler(EventPageRequestCalled))
	assert.True(t, p.hasPageOnHandler(EventPageResponseCalled))
	assert.False(t, p.hasPageOnHandler(EventPageConsoleAPICalled))

	// Remove the handlers and check if they are removed correctly.
	p.removeEventHandler(EventPageRequestCalled, id1)
	p.removeEventHandler(EventPageRequestCalled, id2)
	assert.False(t, p.hasPageOnHandler(EventPageRequestCalled))
	p.removeEventHandler(EventPageResponseCalled, id3)
	assert.False(t, p.hasPageOnHandler(EventPageResponseCalled))
}
