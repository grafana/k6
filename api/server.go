package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
)

func newHandler(cs *v1.ControlSurface) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/", v1.NewHandler(cs))
	mux.Handle("/ping", handlePing(cs.RunState.Logger))
	mux.Handle("/", handlePing(cs.RunState.Logger))
	return mux
}

// GetServer returns a http.Server instance that can serve k6's REST API.
func GetServer(
	runCtx context.Context, addr string, runState *lib.TestRunState,
	samples chan metrics.SampleContainer, me *engine.MetricsEngine, es *execution.Scheduler,
) *http.Server {
	// TODO: reduce the control surface as much as possible? For example, if
	// we refactor the Runner API, we won't need to send the Samples channel.
	cs := &v1.ControlSurface{
		RunCtx:        runCtx,
		Samples:       samples,
		MetricsEngine: me,
		Scheduler:     es,
		RunState:      runState,
	}

	mux := withLoggingHandler(runState.Logger, newHandler(cs))
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
}

type wrappedResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w wrappedResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// withLoggingHandler returns the middleware which logs response status for request.
func withLoggingHandler(l logrus.FieldLogger, next http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		wrapped := wrappedResponseWriter{ResponseWriter: rw, status: 200} // The default status code is 200 if it's not set
		next.ServeHTTP(wrapped, r)

		l.WithField("status", wrapped.status).Debugf("%s %s", r.Method, r.URL.Path)
	}
}

func handlePing(logger logrus.FieldLogger) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
		if _, err := fmt.Fprint(rw, "ok"); err != nil {
			logger.WithError(err).Error("Error while printing ok")
		}
	})
}
