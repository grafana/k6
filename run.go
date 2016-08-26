package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/loadimpact/speedboat/lib"
	"gopkg.in/urfave/cli.v1"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

func guessType(filename string) string {
	switch {
	case strings.Contains(filename, "://"):
		return "url"
	case strings.HasSuffix(filename, ".js"):
		return "js"
	default:
		return ""
	}
}

func makeRunner(filename, t string) (lib.Runner, error) {
	if t == "auto" {
		t = guessType(filename)
	}
	return nil, nil
}

func actionRun(cc *cli.Context) error {
	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}

	filename := args[0]
	runnerType := cc.String("type")
	runner, err := makeRunner(filename, runnerType)
	if err != nil {
		log.WithError(err).Error("Couldn't create a runner")
	}

	engine := lib.Engine{
		Runner: runner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.Debug("Starting engine...")
		if err := engine.Run(ctx); err != nil {
			log.WithError(err).Error("Runtime Error")
		}
	}()

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		c.Next()
		log.WithField("status", c.Writer.Status()).Debugf("%s %s", c.Request.Method, path)
	})
	router.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Size() == 0 && len(c.Errors) > 0 {
			c.JSON(c.Writer.Status(), c.Errors)
		}
	})
	v1 := router.Group("/v1")
	{
		v1.GET("/info", func(c *gin.Context) {
			c.JSON(200, gin.H{"version": cc.App.Version})
		})
		v1.GET("/state", func(c *gin.Context) {
			c.JSON(200, engine.State)
		})
		v1.POST("/state/abort", func(c *gin.Context) {
			cancel()
			c.JSON(202, gin.H{"success": true})
		})
		v1.POST("/state/scale", func(c *gin.Context) {
			vus, err := strconv.ParseInt(c.Query("vus"), 10, 64)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			if err := engine.Scale(vus); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			c.JSON(202, gin.H{"success": true})
		})
	}
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "Not Found"})
	})
	router.Run(cc.String("address"))

	return nil
}
