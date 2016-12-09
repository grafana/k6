package api

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
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
	mux.HandleFunc("/ping", HandlePing)

	n := negroni.New()
	n.Use(negroni.NewRecovery())
	n.UseFunc(WithEngine(engine))
	n.UseFunc(NewLogger(log.StandardLogger()))
	n.UseHandler(mux)

	return http.ListenAndServe(addr, n)
}

func NewLogger(l *log.Logger) negroni.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		next(rw, r)

		res := rw.(negroni.ResponseWriter)
		l.WithField("status", res.Status()).Debugf("%s %s", r.Method, r.URL.Path)
	}
}

func WithEngine(engine *lib.Engine) negroni.HandlerFunc {
	return negroni.HandlerFunc(func(rw http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		r = r.WithContext(common.WithEngine(r.Context(), engine))
		next(rw, r)
	})
}

func HandlePing(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(rw, "ok")
}
