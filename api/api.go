package api

import (
	"context"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/loadimpact/speedboat/lib"
	"github.com/manyminds/api2go"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/tylerb/graceful.v1"
	"io/ioutil"
	"net/http"
	// "strconv"
	"time"
)

var contentType = "application/vnd.api+json"

type Server struct {
	Engine *lib.Engine
	Info   lib.Info
}

// Run runs the API server.
// I'm not sure how idiomatic this is, probably not particularly...
func (s *Server) Run(ctx context.Context, addr string) {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(s.logRequestsMiddleware)
	router.Use(s.jsonErrorsMiddleware)

	router.Use(static.Serve("/", static.LocalFile("web/dist", true)))
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
	}
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "Not Found"})
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
