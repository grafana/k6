package v2

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func NewHandler() http.Handler {
	router := httprouter.New()
	return router
}
