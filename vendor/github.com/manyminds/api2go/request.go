package api2go

import "net/http"

// Request contains additional information for FindOne and Find Requests
type Request struct {
	PlainRequest *http.Request
	QueryParams  map[string][]string
	Pagination   map[string]string
	Header       http.Header
	Context      APIContexter
}
