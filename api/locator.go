package api

import "github.com/dop251/goja"

// Locator represents a way to find element(s) on a page at any moment.
type Locator interface {
	// Click on an element using locator's selector with strict mode on.
	Click(opts goja.Value)
	// Dblclick double clicks on an element using locator's selector with strict mode on.
	Dblclick(opts goja.Value)
	// Check element using locator's selector with strict mode on.
	Check(opts goja.Value)
	// Uncheck element using locator's selector with strict mode on.
	Uncheck(opts goja.Value)
}
