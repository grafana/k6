package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/lib"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var commandRun = cli.Command{
	Name:      "run",
	Usage:     "Starts running a load test",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "vus, u",
			Usage: "virtual users to simulate",
			Value: 10,
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "test duration, 0 to run until cancelled",
			Value: 10 * time.Second,
		},
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js",
			Value: "auto",
		},
	},
	Action: actionRun,
}

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
	wg := sync.WaitGroup{}

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

	engine := &lib.Engine{
		Runner: runner,
	}
	engineC, cancelEngine := context.WithCancel(context.Background())

	api := &APIServer{
		Engine: engine,
		Cancel: cancelEngine,
		Info: lib.Info{
			Version: cc.App.Version,
		},
	}
	apiC, cancelAPI := context.WithCancel(context.Background())

	timeout := cc.Duration("duration")
	if timeout > 0 {
		engineC, _ = context.WithTimeout(engineC, timeout)
	}

	wg.Add(2)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.Debug("Starting engine...")
		if err := engine.Run(engineC); err != nil {
			log.WithError(err).Error("Runtime Error")
		}
	}()
	go func() {
		defer func() {
			log.Debug("API Server terminated")
			wg.Done()
		}()

		addr := cc.GlobalString("address")
		log.WithField("addr", addr).Debug("API Server starting...")
		api.Run(apiC, addr)
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	log.WithField("signal", sig).Debug("Signal received; shutting down...")

	cancelAPI()
	cancelEngine()
	wg.Wait()

	return nil
}
