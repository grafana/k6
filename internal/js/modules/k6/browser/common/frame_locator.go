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

// GetByAltText creates and returns a new locator for this frame locator
// based on the alt attribute text.
func (fl *FrameLocator) GetByAltText(alt string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByAltText", "selector: %q alt: %q opts:%+v", fl.selector, alt, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("alt", alt, opts), nil)
}

// GetByLabel creates and returns a new locator for this frame locator based on the label text.
func (fl *FrameLocator) GetByLabel(label string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByLabel", "selector: %q label: %q opts:%+v", fl.selector, label, opts)

	return fl.Locator(fl.frame.buildLabelSelector(label, opts), nil)
}

// GetByPlaceholder creates and returns a new locator for this frame locator based on the placeholder attribute.
func (fl *FrameLocator) GetByPlaceholder(placeholder string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByPlaceholder", "selector: %q placeholder: %q opts:%+v", fl.selector, placeholder, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("placeholder", placeholder, opts), nil)
}

// GetByRole creates and returns a new locator for this frame locator based on their ARIA role.
func (fl *FrameLocator) GetByRole(role string, opts *GetByRoleOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByRole", "selector: %q role: %q opts:%+v", fl.selector, role, opts)

	return fl.Locator(fl.frame.buildRoleSelector(role, opts), nil)
}

// GetByTestID creates and returns a new locator for this frame locator based on the data-testid attribute.
func (fl *FrameLocator) GetByTestID(testID string) *Locator {
	fl.log.Debugf("FrameLocator:GetByTestID", "selector: %q testID: %q", fl.selector, testID)

	return fl.Locator(fl.frame.buildTestIDSelector(testID), nil)
}

// GetByText creates and returns a new locator for this frame locator based on text content.
func (fl *FrameLocator) GetByText(text string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByText", "selector: %q text: %q opts:%+v", fl.selector, text, opts)

	return fl.Locator(fl.frame.buildTextSelector(text, opts), nil)
}

// GetByTitle creates and returns a new locator for this frame locator based on the title attribute.
func (fl *FrameLocator) GetByTitle(title string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByTitle", "selector: %q title: %q opts:%+v", fl.selector, title, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("title", title, opts), nil)
}

// Locator creates and returns a new locator chained/relative to the current FrameLocator.
func (fl *FrameLocator) Locator(selector string, opts *LocatorOptions) *Locator {
	// Add frame navigation marker to indicate we need to enter the frame's contentDocument
	frameNavSelector := fl.selector + " >> internal:control=enter-frame >> " + selector
	return NewLocator(fl.ctx, opts, frameNavSelector, fl.frame, fl.log)
}

// FrameLocator creates a nested frame locator for an iframe matching the given
func (fl *FrameLocator) FrameLocator(selector string) *FrameLocator {
	fl.log.Debugf("FrameLocator:FrameLocator", "selector:%q childSelector:%q", fl.selector, selector)

	return fl.Locator(selector, nil).ContentFrame()
}
