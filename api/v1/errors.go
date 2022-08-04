package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Error is an api error
type Error struct {
	Status string `json:"status,omitempty"`
	Title  string `json:"title,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}

// ErrorResponse is a struct wrapper around multiple errors
type ErrorResponse struct {
	Errors []Error `json:"errors"`
}

func apiError(rw http.ResponseWriter, title, detail string, status int) {
	doc := ErrorResponse{
		Errors: []Error{
			{
				Status: strconv.Itoa(status),
				Title:  title,
				Detail: detail,
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	rw.WriteHeader(status)
	_, _ = rw.Write(data)
}
