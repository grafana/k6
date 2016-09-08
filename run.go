package main

import (
	"context"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/api"
	"github.com/loadimpact/speedboat/lib"
	"github.com/loadimpact/speedboat/simple"
	"gopkg.in/guregu/null.v3"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	TypeAuto = "auto"
	TypeURL  = "url"
	TypeJS   = "js"
)

var (
	ErrUnknownType = errors.New("Unable to infer type from argument; specify with -t/--type")
	ErrInvalidType = errors.New("Invalid type specified, see --help")
)

var commandRun = cli.Command{
	Name:      "run",
	Usage:     "Starts running a load test",
	ArgsUsage: "url|filename",
	Flags: []cli.Flag{
		cli.Int64Flag{
			Name:  "vus, u",
			Usage: "virtual users to simulate",
			Value: 10,
		},
		cli.DurationFlag{
			Name:  "duration, d",
			Usage: "test duration, 0 to run until cancelled",
			Value: 10 * time.Second,
		},
		cli.Int64Flag{
			Name:  "prepare, p",
			Usage: "VUs to prepare (but not start)",
			Value: 0,
		},
		cli.StringFlag{
			Name:  "type, t",
			Usage: "input type, one of: auto, url, js",
			Value: "auto",
		},
		cli.BoolFlag{
			Name:  "quit, q",
			Usage: "quit immediately on test completion",
		},
	},
	Action: actionRun,
}

func guessType(filename string) string {
	switch {
	case strings.Contains(filename, "://"):
		return TypeURL
	case strings.HasSuffix(filename, ".js"):
		return TypeJS
	default:
		return ""
	}
}

func makeRunner(filename, t string) (lib.Runner, error) {
	if t == TypeAuto {
		t = guessType(filename)
	}

	switch t {
	case TypeAuto:
		return makeRunner(filename, t)
	case "":
		return nil, ErrUnknownType
	case TypeURL:
		return simple.New(filename)
	default:
		return nil, ErrInvalidType
	}
}

func actionRun(cc *cli.Context) error {
	wg := sync.WaitGroup{}

	args := cc.Args()
	if len(args) != 1 {
		return cli.NewExitError("Wrong number of arguments!", 1)
	}

	// Collect arguments
	addr := cc.GlobalString("address")

	duration := cc.Duration("duration")
	vus := cc.Int64("vus")

	prepared := cc.Int64("prepare")
	if prepared == 0 {
		prepared = vus
	}

	quit := cc.Bool("quit")

	// Make the Runner
	filename := args[0]
	runnerType := cc.String("type")
	runner, err := makeRunner(filename, runnerType)
	if err != nil {
		log.WithError(err).Error("Couldn't create a runner")
		return err
	}

	// Make the Engine
	engine, err := lib.NewEngine(runner, prepared)
	if err != nil {
		log.WithError(err).Error("Couldn't create the engine")
		return err
	}
	engineC, engineCancel := context.WithCancel(context.Background())

	// Make the API Server
	srv := &api.Server{
		Engine: engine,
		Info:   lib.Info{Version: cc.App.Version},
	}
	srvC, srvCancel := context.WithCancel(context.Background())

	// Make the Client
	cl, err := api.NewClient(addr)
	if err != nil {
		log.WithError(err).Error("Couldn't make a client; is the address valid?")
		return err
	}

	// Run the engine and API server in the background
	wg.Add(2)
	go func() {
		defer func() {
			log.Debug("Engine terminated")
			wg.Done()
		}()
		log.WithField("prepared", prepared).Debug("Starting engine...")
		if err := engine.Run(engineC); err != nil {
			log.WithError(err).Error("Engine Error")
		}
	}()
	go func() {
		defer func() {
			log.Debug("API Server terminated")
			wg.Done()
		}()
		log.WithField("addr", addr).Debug("API Server starting...")
		srv.Run(srvC, addr)
	}()

	// Wait for the API server to come online
	startTime := time.Now()
	for {
		if err := cl.Ping(); err != nil {
			if time.Since(startTime) < 1*time.Second {
				log.WithError(err).Debug("Waiting for API server to start...")
				time.Sleep(1 * time.Millisecond)
			} else {
				log.WithError(err).Warn("Connection to API server failed; retrying...")
				time.Sleep(1 * time.Second)
			}
			continue
		}
		break
	}

	// Scale the test up to the desired VU count
	if vus > 0 {
		log.WithField("vus", vus).Debug("Starting test...")
		status := lib.Status{
			Running:   null.BoolFrom(true),
			ActiveVUs: null.IntFrom(vus),
		}
		if _, err := cl.UpdateStatus(status); err != nil {
			log.WithError(err).Error("Couldn't scale test")
		}
	}

	// Pause the test once the duration expires
	if duration > 0 {
		log.WithField("duration", duration).Debug("Test will pause after...")
		go func() {
			time.Sleep(duration)
			log.Debug("Duration expired, pausing...")
			status := lib.Status{Running: null.BoolFrom(false)}
			if _, err := cl.UpdateStatus(status); err != nil {
				log.WithError(err).Error("Couldn't pause test")
			}

			if quit {
				log.Debug("Quit requested, terminating...")
				srvCancel()
			}
		}()
	}

	// Wait for a signal or timeout before shutting down
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	log.Debug("Waiting for test to finish")
	select {
	case <-srvC.Done():
		log.Debug("API server terminated; shutting down...")
	case sig := <-signals:
		log.WithField("signal", sig).Debug("Signal received; shutting down...")
	}

	// Shut down the API server and engine, wait for them to terminate before exiting
	srvCancel()
	engineCancel()
	wg.Wait()

	return nil
}
