// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"errors"
	"io/fs"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	pathEvents = "/events"
	pathUI     = "/ui/"
	pathReport = "/report"

	eventChannel    = "events"
	snapshotEvent   = "snapshot"
	cumulativeEvent = "cumulative"
	startEvent      = "start"
	stopEvent       = "stop"
	configEvent     = "config"
	metricEvent     = "metric"
	paramEvent      = "param"
	thresholdEvent  = "threshold"
)

type webServer struct {
	*eventEmitter
	*http.ServeMux
	server *http.Server
}

func newWebServer(
	uiFS fs.FS,
	reportHandler http.Handler,
	logger logrus.FieldLogger,
) *webServer { //nolint:ireturn
	srv := &webServer{
		eventEmitter: newEventEmitter(eventChannel, logger),
		ServeMux:     http.NewServeMux(),
	}

	srv.Handle(pathEvents, srv.eventEmitter)
	srv.Handle(pathUI, http.StripPrefix(pathUI, http.FileServer(http.FS(uiFS))))
	srv.Handle(pathReport, reportHandler)

	srv.HandleFunc("/", rootHandler(pathUI))

	srv.server = &http.Server{
		Handler:           srv.ServeMux,
		ReadHeaderTimeout: time.Second,
	} //nolint:exhaustruct

	return srv
}

func (srv *webServer) listenAndServe(addr string) (*net.TCPAddr, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		serr := srv.server.Serve(listener)
		if serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			srv.logger.Error(serr)
		}
	}()

	a, _ := listener.Addr().(*net.TCPAddr)

	return a, nil
}

func (srv *webServer) stop() error {
	srv.eventEmitter.Close()

	err := srv.server.Close()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func rootHandler(uiPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { //nolint:varnamelen
		if r.URL.Path == "/" {
			http.Redirect(
				w,
				r,
				path.Join(uiPath, r.URL.Path)+"?endpoint=/",
				http.StatusTemporaryRedirect,
			)

			return
		}

		http.NotFound(w, r)
	}
}
