package common

import (
	"context"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// FrameLocator represent a way to find element(s) in an iframe.
type FrameLocator struct {
	selector string

	frame *Frame

	ctx context.Context
	log *log.Logger
}

// NewFrameLocator creates and returns a new frame locator.
func NewFrameLocator(ctx context.Context, selector string, f *Frame, l *log.Logger) *FrameLocator {
	return &FrameLocator{
		selector: selector,
		frame:    f,
		ctx:      ctx,
		log:      l,
	}
}

// Locator creates and returns a new locator chained/relative to the current FrameLocator.
func (fl *FrameLocator) Locator(selector string) *Locator {
	// Add frame navigation marker to indicate we need to enter the frame's contentDocument
	frameNavSelector := fl.selector + " >> internal:control=enter-frame >> " + selector
	return NewLocator(fl.ctx, nil, frameNavSelector, fl.frame, fl.log)
}
