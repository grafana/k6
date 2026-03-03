// Package api contains the REST API implementation for k6.
// It also registers the services endpoints like pprof
package api

import (
	"context"
	"expvar"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // Register pprof handlers
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/metrics/engine"
	"go.k6.io/k6/internal/observability/jsexec"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func newHandler(cs *v1.ControlSurface, profilingEnabled bool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/", v1.NewHandler(cs))
	mux.Handle("/ping", handlePing(cs.RunState.Logger))
	mux.Handle("/", handlePing(cs.RunState.Logger))

	injectProfilerHandler(mux, profilingEnabled)

	return mux
}

func injectProfilerHandler(mux *http.ServeMux, profilingEnabled bool) {
	var handler http.Handler

	handler = http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
		_, _ = rw.Write([]byte("To enable profiling, please run k6 with the --profiling-enabled flag"))
	})

	if profilingEnabled {
		handler = http.DefaultServeMux
	}

	mux.Handle("/debug/pprof/", handler)
	mux.Handle("/debug/pprof/js-cpu", jsArtifactHandler("js-cpu", "application/octet-stream"))
	mux.Handle("/debug/pprof/js-trace", jsArtifactHandler("js-trace", "application/octet-stream"))
	mux.Handle("/debug/vars/", expvar.Handler())
	mux.Handle("/metrics", promhttp.Handler())
}

func jsArtifactHandler(name, ctype string) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		artifactName := name
		if name == "js-cpu" {
			switch r.URL.Query().Get("scope") {
			case "init":
				artifactName = "js-cpu-init"
			case "vu":
				artifactName = "js-cpu-vu"
			case "combined":
				artifactName = "js-cpu-combined"
			}
		}
		artifact, ok := jsexec.LatestArtifact(artifactName)
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte("no artifact captured yet"))
			return
		}
		rw.Header().Set("Content-Type", ctype)
		rw.Header().Set("X-K6-JS-Profile-ID", artifact.ProfileID)
		rw.Header().Set("X-K6-JS-Artifact-Created-At", artifact.CreatedAt.Format(time.RFC3339Nano))
		_, _ = rw.Write(artifact.Data)
	})
}

// GetServer returns a http.Server instance that can serve k6's REST API.
func GetServer(
	runCtx context.Context,
	addr string,
	profilingEnabled bool,
	runState *lib.TestRunState,
	samples chan metrics.SampleContainer,
	me *engine.MetricsEngine,
	es *execution.Scheduler,
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

	mux := withLoggingHandler(runState.Logger, newHandler(cs, profilingEnabled))
	return &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
}

type wrappedResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *wrappedResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(w.status)
}

// withLoggingHandler returns the middleware which logs response status for request.
func withLoggingHandler(l logrus.FieldLogger, next http.Handler) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		wrapped := &wrappedResponseWriter{ResponseWriter: rw, status: 200} // The default status code is 200 if it's not set
		next.ServeHTTP(wrapped, r)

		l.WithField("status", wrapped.status).Debugf("%s %s", r.Method, r.URL.Path)
	}
}

func handlePing(logger logrus.FieldLogger) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
		if _, err := fmt.Fprint(rw, "ok"); err != nil {
			logger.WithError(err).Error("Error while printing ok")
		}
	})
}
