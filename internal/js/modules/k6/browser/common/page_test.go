package common

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		eventHandlers: make(map[PageEventName][]pageEventHandlerRecord),
	}

	// Add some handlers to the page event handling system.
	handler := func(PageEvent) error { return nil }
	id1, err := p.addEventHandler(PageEventRequest, handler)
	assert.NoError(t, err)
	id2, err := p.addEventHandler(PageEventRequest, handler)
	assert.NoError(t, err)
	id3, err := p.addEventHandler(PageEventResponse, handler)
	assert.NoError(t, err)

	// Check if the handlers are registered correctly.
	assert.True(t, p.hasEventHandler(PageEventRequest))
	assert.True(t, p.hasEventHandler(PageEventResponse))
	assert.False(t, p.hasEventHandler(PageEventConsole))

	// Remove the handlers and check if they are removed correctly.
	p.removeEventHandler(PageEventRequest, id1)
	p.removeEventHandler(PageEventRequest, id2)
	assert.False(t, p.hasEventHandler(PageEventRequest))
	p.removeEventHandler(PageEventResponse, id3)
	assert.False(t, p.hasEventHandler(PageEventResponse))
}

func TestPageEventHandlerIterator(t *testing.T) {
	t.Parallel()

	newPage := func() *Page {
		return &Page{
			eventHandlers: make(map[PageEventName][]pageEventHandlerRecord),
		}
	}

	t.Run("zero", func(t *testing.T) {
		t.Parallel()

		n := 0
		for range newPage().eventHandlersByName(PageEventConsole) {
			n++
		}
		assert.Zero(t, n, "must not yield any handlers")
	})

	t.Run("single_handler", func(t *testing.T) {
		t.Parallel()

		var called bool

		page := newPage()
		_, err := page.addEventHandler(PageEventConsole, func(event PageEvent) error {
			called = true
			return nil
		})
		require.NoError(t, err)
		for handle := range page.eventHandlersByName(PageEventConsole) {
			_ = handle(PageEvent{})
		}
		assert.Truef(t, called, "did not call the registered handler")
	})

	t.Run("multiple_handlers", func(t *testing.T) {
		t.Parallel()

		var called []int

		p := newPage()
		_, err := p.addEventHandler(PageEventResponse, func(event PageEvent) error {
			called = append(called, 1)
			return nil
		})
		require.NoError(t, err)
		_, err = p.addEventHandler(PageEventResponse, func(event PageEvent) error {
			called = append(called, 2)
			return nil
		})
		require.NoError(t, err)

		for handle := range p.eventHandlersByName(PageEventResponse) {
			_ = handle(PageEvent{})
		}
		assert.Lenf(t, called, 2, "must call all registered handlers")
		assert.Equal(t, []int{1, 2}, called, "must call handlers in order of registration")
	})

	t.Run("handler_error", func(t *testing.T) {
		t.Parallel()

		handlerError := errors.New("handler error")

		page := newPage()
		_, err := page.addEventHandler(PageEventRequest, func(event PageEvent) error {
			return handlerError
		})
		require.NoError(t, err)

		for handle := range page.eventHandlersByName(PageEventRequest) {
			assert.ErrorIs(t, handle(PageEvent{}), handlerError)
		}
	})
}

func TestPageOn(t *testing.T) {
	t.Parallel()

	t.Run("nil handler", func(t *testing.T) {
		t.Parallel()

		p := &Page{}

		err := p.On("metric", nil)
		assert.Error(t, err)
		assert.ErrorContains(t, err, `"handler" argument cannot be nil`)
	})

	t.Run("valid handler", func(t *testing.T) {
		t.Parallel()

		p := &Page{
			eventHandlers: make(map[PageEventName][]pageEventHandlerRecord),
		}
		handler := func(PageEvent) error { return nil }

		err := p.On("metric", handler)
		assert.NoError(t, err)
		assert.Len(t, p.eventHandlers[("metric")], 1)
	})
}
