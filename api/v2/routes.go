package v2

import (
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func NewHandler() http.Handler {
	router := httprouter.New()
	router.GET("/v2/status", HandleGetStatus)
	router.GET("/v2/metrics", HandleGetMetrics)
	router.GET("/v2/metrics/:id", HandleGetMetric)
	return router
}
