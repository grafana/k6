package api

import (
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/api/v1"
	"github.com/loadimpact/k6/api/v2"
	"github.com/loadimpact/k6/lib"
	"github.com/urfave/negroni"
	"net/http"
)

func ListenAndServe(addr string, engine *lib.Engine) error {
	mux := http.NewServeMux()
	mux.Handle("/v1/", v1.NewHandler())
	mux.Handle("/v2/", v2.NewHandler())

	n := negroni.Classic()
	n.UseFunc(WithEngine(engine))
	n.UseHandler(mux)
	return http.ListenAndServe(addr, n)
}

func WithEngine(engine *lib.Engine) negroni.HandlerFunc {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		r = r.WithContext(common.WithEngine(r.Context(), engine))
		next(rw, r)
	})
}
