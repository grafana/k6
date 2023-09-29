package api

// ConsoleMessageAPI represents a page console message.
type ConsoleMessageAPI struct {
	// Args represent the list of arguments passed to a console function call.
	Args []JSHandleAPI

	// Page is the page that produced the console message, if any.
	Page PageAPI

	// Text represents the text of the console message.
	Text string

	// Type is the type of the console message.
	// It can be one of 'log', 'debug', 'info', 'error', 'warning', 'dir', 'dirxml',
	// 'table', 'trace', 'clear', 'startGroup', 'startGroupCollapsed', 'endGroup',
	// 'assert', 'profile', 'profileEnd', 'count', 'timeEnd'.
	Type string
}
