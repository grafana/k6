package api

import "github.com/dop251/goja"

// Locator represents a way to find element(s) on a page at any moment.
type Locator interface {
	// Click on an element using locator's selector with strict mode on.
	Click(opts goja.Value)
}
