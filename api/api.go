package api

import (
	"github.com/loadimpact/speedboat/api/global"
	"github.com/loadimpact/speedboat/api/http"
)

type RegisterFunc func() map[string]interface{}

var API = map[string]RegisterFunc{
	"global": global.New,
	"http":   http.New,
}

func New() map[string]map[string]interface{} {
	res := make(map[string]map[string]interface{})
	for name, factory := range API {
		res[name] = factory()
	}
	return res
}
