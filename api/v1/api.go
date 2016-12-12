package v1

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/lib"
	"github.com/manyminds/api2go"
	"github.com/manyminds/api2go/jsonapi"
	"io/ioutil"
	"net/http"
	"strconv"
)

var contentType = "application/vnd.api+json"

func NewHandler() http.Handler {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(jsonErrorsMiddleware)

	v1 := router.Group("/v1")
	{
		v1.GET("/info", func(c *gin.Context) {
			data, err := jsonapi.Marshal(lib.Info{})
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/error", func(c *gin.Context) {
			_ = c.AbortWithError(500, errors.New("This is an error"))
		})
		v1.GET("/status", func(c *gin.Context) {
			engine := common.GetEngine(c)
			data, err := jsonapi.Marshal(engine.Status)
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.PATCH("/status", func(c *gin.Context) {
			engine := common.GetEngine(c)

			var status lib.Status
			data, _ := ioutil.ReadAll(c.Request.Body)
			if err := jsonapi.Unmarshal(data, &status); err != nil {
				_ = c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			if status.VUsMax.Valid {
				if status.VUsMax.Int64 < engine.Status.VUs.Int64 {
					if status.VUsMax.Int64 >= status.VUs.Int64 {
						if err := engine.SetVUs(status.VUs.Int64); err != nil {
							_ = c.AbortWithError(http.StatusBadRequest, err)
							return
						}
					} else {
						_ = c.AbortWithError(http.StatusBadRequest, lib.ErrMaxTooLow)
						return
					}
				}

				if err := engine.SetMaxVUs(status.VUsMax.Int64); err != nil {
					_ = c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
			if status.VUs.Valid {
				if status.VUs.Int64 > engine.Status.VUsMax.Int64 {
					_ = c.AbortWithError(http.StatusBadRequest, lib.ErrTooManyVUs)
					return
				}

				if err := engine.SetVUs(status.VUs.Int64); err != nil {
					_ = c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
			}
			if status.Running.Valid {
				engine.SetRunning(status.Running.Bool)
			}

			data, err := jsonapi.Marshal(engine.Status)
			if err != nil {
				_ = c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/metrics", func(c *gin.Context) {
			engine := common.GetEngine(c)
			metrics := make([]interface{}, 0, len(engine.Metrics))
			for metric, sink := range engine.Metrics {
				metric.Sample = sink.Format()
				metrics = append(metrics, metric)
			}
			data, err := jsonapi.Marshal(metrics)
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/metrics/:id", func(c *gin.Context) {
			engine := common.GetEngine(c)
			id := c.Param("id")
			for metric, sink := range engine.Metrics {
				if metric.Name != id {
					continue
				}
				metric.Sample = sink.Format()
				data, err := jsonapi.Marshal(metric)
				if err != nil {
					_ = c.AbortWithError(500, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			_ = c.AbortWithError(404, errors.New("Metric not found"))
		})
		v1.GET("/groups", func(c *gin.Context) {
			engine := common.GetEngine(c)
			data, err := jsonapi.Marshal(engine.Runner.GetGroups())
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/groups/:id", func(c *gin.Context) {
			engine := common.GetEngine(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				_ = c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			for _, group := range engine.Runner.GetGroups() {
				if group.ID != id {
					continue
				}

				data, err := jsonapi.Marshal(group)
				if err != nil {
					_ = c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			_ = c.AbortWithError(404, errors.New("Group not found"))
		})
		v1.GET("/checks", func(c *gin.Context) {
			engine := common.GetEngine(c)
			data, err := jsonapi.Marshal(engine.Runner.GetChecks())
			if err != nil {
				_ = c.AbortWithError(500, err)
				return
			}
			c.Data(200, contentType, data)
		})
		v1.GET("/checks/:id", func(c *gin.Context) {
			engine := common.GetEngine(c)
			id, err := strconv.ParseInt(c.Param("id"), 10, 64)
			if err != nil {
				_ = c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			for _, check := range engine.Runner.GetChecks() {
				if check.ID != id {
					continue
				}

				data, err := jsonapi.Marshal(check)
				if err != nil {
					_ = c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				c.Data(200, contentType, data)
				return
			}
			_ = c.AbortWithError(404, errors.New("Group not found"))
		})
	}

	return router
}

func jsonErrorsMiddleware(c *gin.Context) {
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
