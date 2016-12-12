package v2

import (
	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	"github.com/manyminds/api2go/jsonapi"
	"net/http"
)

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

func HandleGetMetric(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	engine := common.GetEngine(r.Context())

	var metric Metric
	var found bool
	for m, _ := range engine.Metrics {
		if m.Name == id {
			metric = NewMetric(*m)
			found = true
			break
		}
	}

	if !found {
		http.Error(rw, "No such metric", http.StatusNotFound)
		return
	}

	data, err := jsonapi.Marshal(metric)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
