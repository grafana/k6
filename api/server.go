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
	"fmt"
	"net/http"

	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/api/v1"
	"github.com/loadimpact/k6/lib"
	"github.com/urfave/negroni"
)

var static = rice.MustFindBox("../web/dist")

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/", v1.NewHandler())
	mux.Handle("/ping", HandlePing())
	mux.Handle("/", http.FileServer(static.HTTPBox()))
	return mux
}

func ListenAndServe(addr string, engine *lib.Engine) error {
	mux := NewHandler()

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

func HandlePing() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(rw, "ok")
	})
}
