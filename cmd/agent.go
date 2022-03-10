package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/distributed"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
	"google.golang.org/grpc"
	"gopkg.in/guregu/null.v3"
)

// TODO: something cleaner
func getMetricsHook(
	ctx context.Context, instanceID uint32,
	client distributed.DistributedTestClient, logger logrus.FieldLogger,
) func(*engine.MetricsEngine) func() {
	logger = logger.WithField("component", "metric-engine-hook")
	return func(me *engine.MetricsEngine) func() {
		stop := make(chan struct{})
		done := make(chan struct{})

		dumpMetrics := func() {
			logger.Debug("Starting metric dump...")
			me.MetricsLock.Lock()
			defer me.MetricsLock.Unlock()

			metrics := make([]*distributed.MetricDump, 0, len(me.ObservedMetrics))
			for _, om := range me.ObservedMetrics {
				data, err := om.Sink.Drain()
				if err != nil {
					logger.Errorf("There was a problem draining the sink for metric %s: %s", om.Name, err)
				}
				metrics = append(metrics, &distributed.MetricDump{
					Name: om.Name,
					Data: data,
				})
			}

			data := &distributed.MetricsDump{
				InstanceID: instanceID,
				Metrics:    metrics,
			}
			_, err := client.SendMetrics(ctx, data)
			if err != nil {
				logger.Errorf("There was a problem dumping metrics: %s", err)
			}
		}

		go func() {
			defer close(done)
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					dumpMetrics()
				case <-stop:
					dumpMetrics()
					return
				}
			}
		}()

		finalize := func() {
			logger.Debug("Final metric dump...")
			close(stop)
			<-done
			logger.Debug("Done!")
		}

		return finalize
	}
}

// TODO: a whole lot of cleanup, refactoring, error handling and hardening
func getCmdAgent(gs *globalState) *cobra.Command { //nolint: funlen
	c := &cmdsRunAndAgent{gs: gs}

	c.loadTest = func(cmd *cobra.Command, args []string) (*loadedTest, execution.Controller, error) {
		conn, err := grpc.Dial(args[0], grpc.WithInsecure())
		if err != nil {
			return nil, nil, err
		}
		c.testEndHook = func(err error) {
			gs.logger.Debug("k6 agent run ended with err=%s", err)
			conn.Close()
		}

		client := distributed.NewDistributedTestClient(conn)

		resp, err := client.Register(gs.ctx, &distributed.RegisterRequest{})
		if err != nil {
			return nil, nil, err
		}

		c.metricsEngineHook = getMetricsHook(gs.ctx, resp.InstanceID, client, gs.logger)

		controller, err := distributed.NewAgentController(gs.ctx, resp.InstanceID, client, gs.logger)
		if err != nil {
			return nil, nil, err
		}

		var options lib.Options
		if err := json.Unmarshal(resp.Options, &options); err != nil {
			return nil, nil, err
		}

		arc, err := lib.ReadArchive(bytes.NewReader(resp.Archive))
		if err != nil {
			return nil, nil, err
		}

		registry := metrics.NewRegistry()
		builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
		rtOpts := lib.RuntimeOptions{
			NoThresholds:      null.BoolFrom(true),
			NoSummary:         null.BoolFrom(true),
			Env:               arc.Env,
			CompatibilityMode: null.StringFrom(arc.CompatibilityMode),
		}
		initRunner, err := js.NewFromArchive(gs.logger, arc, rtOpts, builtinMetrics, registry)
		if err != nil {
			return nil, nil, err
		}

		// Hacks to get the default config values...
		flagSet := c.flagSet()
		flagSet.Parse([]string{}) // use the
		defaults, err := getConfig(flagSet)
		if err != nil {
			return nil, nil, err
		}
		pseudoConsoldatedConfig := defaults.Apply(Config{Options: options})
		for _, thresholds := range pseudoConsoldatedConfig.Thresholds {
			if err = thresholds.Parse(); err != nil {
				return nil, nil, err
			}
		}
		derivedConfig, err := deriveAndValidateConfig(pseudoConsoldatedConfig, initRunner.IsExecutable, gs.logger)
		if err != nil {
			return nil, nil, err
		}

		test := &loadedTest{
			testPath: arc.Filename,
			source: &loader.SourceData{
				Data: resp.Archive,
				URL:  arc.FilenameURL,
			},
			fileSystems:        arc.Filesystems,
			runtimeOptions:     rtOpts,
			metricsRegistry:    registry,
			builtInMetrics:     builtinMetrics,
			initRunner:         initRunner,
			consolidatedConfig: pseudoConsoldatedConfig,
			derivedConfig:      derivedConfig,
		}

		gs.flags.address = "" // TODO: fix, this is a hack so agents don't start an API server

		return test, controller, nil // TODO
	}

	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Join a distributed load test",
		Long:  `TODO`,
		Args:  exactArgsWithMsg(1, "arg should either the IP and port of the controller k6 instance"),
		RunE:  c.run,
	}

	// TODO: add flags

	return agentCmd
}
