/*

k6 - a next-generation load testing tool
Copyright (C) 2016 Load Impact

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/GeertJohan/go.rice"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/loadimpact/k6/lib"
	"github.com/manyminds/api2go"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/tylerb/graceful.v1"
	"io/ioutil"
	"mime"
	"net/http"
	"path"
	"strconv"
	// "strconv"
	"time"
)

var (
	contentType = "application/vnd.api+json"
	webBox      = rice.MustFindBox("../web/dist")
)

type Server struct {
	Engine *lib.Engine
	Info   lib.Info
}

// Run runs the API server.
// I'm not sure how idiomatic this is, probably not particularly...
func (s *Server) Run(ctx context.Context, addr string) {
	indexData, err := webBox.Bytes("index.html")
	if err != nil {
		log.WithError(err).Error("Couldn't load index.html; web UI unavailable")
	}

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(s.logRequestsMiddleware)
	router.Use(s.jsonErrorsMiddleware)

	// router.Use(static.Serve("/", static.LocalFile("web/dist", true)))
	router.GET("/ping", func(c *gin.Context) {
		c.Data(http.StatusNoContent, "", nil)
	})
	v1 := router.Group("/v1")
	{
		v1.GET("/info", func(c *gin.Context) {
			data, err := jsonapi.Marshal(s.Info)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/error", func(c *gin.Context) {
			c.AbortWithError(500, errors.New("This is an error"))
		})
		v1.GET("/status", func(c *gin.Context) {
			data, err := jsonapi.Marshal(s.Engine.Status)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.PATCH("/status", func(c *gin.Context) {
			var status lib.Status
			data, _ := ioutil.ReadAll(c.Request.Body)
			if err := jsonapi.Unmarshal(data, &status); err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			if status.VUsMax.Valid {
				if status.VUsMax.Int64 < s.Engine.Status.VUs.Int64 {
					if status.VUsMax.Int64 >= status.VUs.Int64 {
						s.Engine.SetVUs(status.VUs.Int64)
					} else {
						c.AbortWithError(http.StatusBadRequest, lib.ErrMaxTooLow)
						return
					}
				}

				if err := s.Engine.SetMaxVUs(status.VUsMax.Int64); err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
			if status.VUs.Valid {
				if status.VUs.Int64 > s.Engine.Status.VUsMax.Int64 {
					c.AbortWithError(http.StatusBadRequest, lib.ErrTooManyVUs)
					return
				}

				if err := s.Engine.SetVUs(status.VUs.Int64); err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
			if status.Running.Valid {
				s.Engine.SetRunning(status.Running.Bool)
			}

			data, err := jsonapi.Marshal(s.Engine.Status)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/metrics", func(c *gin.Context) {
			metrics := make([]interface{}, 0, len(s.Engine.Metrics))
			for metric, sink := range s.Engine.Metrics {
				metric.Sample = sink.Format()
				metrics = append(metrics, metric)
			}
			data, err := jsonapi.Marshal(metrics)
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/metrics/:id", func(c *gin.Context) {
			id := c.Param("id")
			for metric, sink := range s.Engine.Metrics {
				if metric.Name != id {
					continue
				}
				metric.Sample = sink.Format()
				data, err := jsonapi.Marshal(metric)
				if err != nil {
					c.AbortWithError(500, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			c.AbortWithError(404, errors.New("Metric not found"))
		})
		v1.GET("/groups", func(c *gin.Context) {
			data, err := jsonapi.Marshal(s.Engine.Runner.GetGroups())
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/groups/:id", func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			for _, group := range s.Engine.Runner.GetGroups() {
				if group.ID != id {
					continue
				}

				data, err := jsonapi.Marshal(group)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			c.AbortWithError(404, errors.New("Group not found"))
		})
		v1.GET("/checks", func(c *gin.Context) {
			data, err := jsonapi.Marshal(s.Engine.Runner.GetChecks())
			if err != nil {
				c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/checks/:id", func(c *gin.Context) {
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			for _, check := range s.Engine.Runner.GetChecks() {
				if check.ID != id {
					continue
				}

				data, err := jsonapi.Marshal(check)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			c.AbortWithError(404, errors.New("Group not found"))
		})
	}
	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		bytes, err := webBox.Bytes(requestPath)
		if err != nil {
			log.WithError(err).Debug("Falling back to index.html")
			if indexData == nil {
				c.String(404, "Web UI is unavailable - see console output.")
				return
			}
			c.Data(200, "text/html; charset=utf-8", indexData)
			return
		}

		mimeType := mime.TypeByExtension(path.Ext(requestPath))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		c.Data(200, mimeType, bytes)
	})

	srv := graceful.Server{NoSignalHandling: true, Server: &http.Server{Addr: addr, Handler: router}}
	go srv.ListenAndServe()

	<-ctx.Done()
	srv.Stop(10 * time.Second)
	<-srv.StopChan()
}

func (s *Server) logRequestsMiddleware(c *gin.Context) {
	path := c.Request.URL.Path
	c.Next()
	log.WithField("status", c.Writer.Status()).Debugf("%s %s", c.Request.Method, path)
}

func (s *Server) jsonErrorsMiddleware(c *gin.Context) {
	c.Header("Content-Type", contentType)
	c.Next()
	if len(c.Errors) > 0 {
		var errors api2go.HTTPError
		for _, err := range c.Errors {
			errors.Errors = append(errors.Errors, api2go.Error{
				Title: err.Error(),
			})
		}
		data, _ := json.Marshal(errors)
		c.Data(c.Writer.Status(), contentType, data)
	}
}
