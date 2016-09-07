package api

import (
	"context"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
	"github.com/loadimpact/speedboat/lib"
	"gopkg.in/tylerb/graceful.v1"
	"net/http"
	// "strconv"
	"time"
)

var contentType = "application/vnd.api+json"

type Server struct {
	Engine *lib.Engine
	Cancel context.CancelFunc

	Info lib.Info
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
			if err := jsonapi.MarshalOnePayload(c.Writer, &s.Info); err != nil {
				c.AbortWithError(500, err)
				return
			}
		})
		v1.GET("/error", func(c *gin.Context) {
			c.AbortWithError(500, errors.New("This is an error"))
		})
		v1.GET("/status", func(c *gin.Context) {
			if err := jsonapi.MarshalOnePayload(c.Writer, &s.Engine.Status); err != nil {
				c.AbortWithError(500, err)
				return
			}
		})
		// v1.GET("/metrics", func(c *gin.Context) {
		// 	metrics := make(map[string]Metric)
		// 	for m, sink := range s.Engine.Metrics {
		// 		metrics[m.Name] = Metric{
		// 			Name:     m.Name,
		// 			Type:     MetricType(m.Type),
		// 			Contains: ValueType(m.Contains),
		// 			Data:     sink.Format(),
		// 		}
		// 	}
		// 	c.JSON(200, metrics)
		// })
		// v1.GET("/metrics/:name", func(c *gin.Context) {
		// 	name := c.Param("name")
		// 	for m, sink := range s.Engine.Metrics {
		// 		if m.Name != name {
		// 			continue
		// 		}

		// 		c.JSON(200, Metric{
		// 			Name:     m.Name,
		// 			Type:     MetricType(m.Type),
		// 			Contains: ValueType(m.Contains),
		// 			Data:     sink.Format(),
		// 		})
		// 		return
		// 	}
		// 	c.AbortWithError(404, errors.New("No such metric"))
		// })
		// v1.POST("/abort", func(c *gin.Context) {
		// 	s.Cancel()
		// 	c.JSON(202, gin.H{"success": true})
		// })
		// v1.POST("/scale", func(c *gin.Context) {
		// 	vus, err := strconv.ParseInt(c.Query("vus"), 10, 64)
		// 	if err != nil {
		// 		c.AbortWithError(http.StatusBadRequest, err)
		// 		return
		// 	}

		// 	if err := s.Engine.Scale(vus); err != nil {
		// 		c.AbortWithError(http.StatusInternalServerError, err)
		// 		return
		// 	}

		// 	c.JSON(202, gin.H{"success": true})
		// })
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
		var errors ErrorResponse
		for _, err := range c.Errors {
			errors.Errors = append(errors.Errors, Error{
				Title: err.Error(),
			})
		}
		data, _ := json.Marshal(errors)
		c.Data(c.Writer.Status(), contentType, data)
	}
}
