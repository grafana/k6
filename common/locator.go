package common

import (
	"context"
)

// Locator represent a way to find element(s) on the page at any moment.
type Locator struct {
	selector string

	frame *Frame

	ctx context.Context
	log *Logger
}

// NewLocator creates and returns a new locator.
func NewLocator(ctx context.Context, selector string, f *Frame, l *Logger) *Locator {
	return &Locator{
		selector: selector,
		frame:    f,
		ctx:      ctx,
		log:      l,
	}
}
