/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/api/common"
	v1 "go.k6.io/k6/api/v1"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/metrics/engine"
	"go.k6.io/k6/stats"
)

func newHandler(cs *common.ControlSurface) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/", v1.NewHandler(cs))
	mux.Handle("/ping", handlePing(cs.Logger))
	mux.Handle("/", handlePing(cs.Logger))
	return mux
}

// NewAPIServer returns a new *unstarted* HTTP REST API server.
func NewAPIServer(
	runCtx context.Context, addr string, samples chan stats.SampleContainer,
	me *engine.MetricsEngine, es *execution.Scheduler, logger logrus.FieldLogger,
) *http.Server {
	// TODO: reduce the control surface as much as possible... For example, if
	// we refactor the Runner API, we won't need to send the Samples channel.
	cs := &common.ControlSurface{
		RunCtx:             runCtx,
		Samples:            samples,
		MetricsEngine:      me,
		ExecutionScheduler: es,
		Logger:             logger,
	}

	return &http.Server{Addr: addr, Handler: newHandler(cs)}
}

type wrappedResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w wrappedResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// newLogger returns the middleware which logs response status for request.
func newLogger(l logrus.FieldLogger, next http.Handler) http.HandlerFunc {
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
