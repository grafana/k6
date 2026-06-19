package common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLocatorPage verifies a locator resolves the page that owns its
// frame. Mirrors the struct-literal style of TestPageLocator; can be
// removed once integration tests cover the locator surface.
func TestLocatorPage(t *testing.T) {
	t.Parallel()

	ctx := context.TODO()
	p := &Page{ctx: ctx}
	fm := &FrameManager{ctx: ctx, page: p}
	f := &Frame{ctx: ctx, manager: fm}

	l := NewLocator(ctx, nil, "span", f, nil)

	assert.Same(t, p, l.Page())
}
