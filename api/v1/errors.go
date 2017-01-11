package v1

import (
	"encoding/json"
	"github.com/manyminds/api2go"
	"net/http"
	"strconv"
)

func apiError(rw http.ResponseWriter, title, detail string, status int) {
	doc := map[string][]api2go.Error{
		"errors": []api2go.Error{
			api2go.Error{
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
