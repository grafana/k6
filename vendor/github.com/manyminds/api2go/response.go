package api2go

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/manyminds/api2go/jsonapi"
)

// The Response struct implements api2go.Responder and can be used as a default
// implementation for your responses
// you can fill the field `Meta` with all the metadata your application needs
// like license, tokens, etc
type Response struct {
	Res        interface{}
	Code       int
	Meta       map[string]interface{}
	Pagination Pagination
}

// Metadata returns additional meta data
func (r Response) Metadata() map[string]interface{} {
	return r.Meta
}

// Result returns the actual payload
func (r Response) Result() interface{} {
	return r.Res
}

// StatusCode sets the return status code
func (r Response) StatusCode() int {
	return r.Code
}

func buildLink(base string, r *http.Request, pagination map[string]string) jsonapi.Link {
	params := r.URL.Query()
	for k, v := range pagination {
		qk := fmt.Sprintf("page[%s]", k)
		params.Set(qk, v)
	}
	if len(params) == 0 {
		return jsonapi.Link{Href: base}
	}
	query, _ := url.QueryUnescape(params.Encode())
	return jsonapi.Link{Href: fmt.Sprintf("%s?%s", base, query)}
}

// Links returns a jsonapi.Links object to include in the top-level response
func (r Response) Links(req *http.Request, baseURL string) (ret jsonapi.Links) {
	ret = make(jsonapi.Links)

	if r.Pagination.Next != nil {
		ret["next"] = buildLink(baseURL, req, r.Pagination.Next)
	}
	if r.Pagination.Prev != nil {
		ret["prev"] = buildLink(baseURL, req, r.Pagination.Prev)
	}
	if r.Pagination.First != nil {
		ret["first"] = buildLink(baseURL, req, r.Pagination.First)
	}
	if r.Pagination.Last != nil {
		ret["last"] = buildLink(baseURL, req, r.Pagination.Last)
	}
	return
}
