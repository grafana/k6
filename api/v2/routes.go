package v2

import (
	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/guregu/null.v3"
	"net/http"
)

func NewHandler() http.Handler {
	router := httprouter.New()
	router.GET("/v2/status", HandleGetStatus)
	router.GET("/v2/metrics", HandleGetMetrics)
	return router
}

func HandleGetStatus(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	status := Status{
		Running: null.BoolFrom(engine.Status.Running.Bool),
		Tainted: null.BoolFrom(engine.Status.Tainted.Bool),
		VUs:     null.IntFrom(engine.Status.VUs.Int64),
		VUsMax:  null.IntFrom(engine.Status.VUsMax.Int64),
	}
	data, err := jsonapi.Marshal(status)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func HandleGetMetrics(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	metrics := make([]Metric, 0)
	for m, _ := range engine.Metrics {
		metrics = append(metrics, NewMetric(*m))
	}

	data, err := jsonapi.Marshal(metrics)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
