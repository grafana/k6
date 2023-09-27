package api

// ConsoleMessage represents a page console message.
type ConsoleMessage struct {
	// Args represent the list of arguments passed to a console function call.
	Args []JSHandle

	// Page is the page that produced the console message, if any.
	Page Page

	// Text represents the text of the console message.
	Text string

	// Type is the type of the console message.
	// It can be one of 'log', 'debug', 'info', 'error', 'warning', 'dir', 'dirxml',
	// 'table', 'trace', 'clear', 'startGroup', 'startGroupCollapsed', 'endGroup',
	// 'assert', 'profile', 'profileEnd', 'count', 'timeEnd'.
	Type string
}
