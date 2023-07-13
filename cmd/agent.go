package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/distributed"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/metrics/engine"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func loadAndConfigureDistTest(gs *state.GlobalState, distTest *distributed.Test) (*loadedAndConfiguredTest, error) {
	var options lib.Options
	if err := json.Unmarshal(distTest.Options, &options); err != nil {
		return nil, err
	}

	arc, err := lib.ReadArchive(bytes.NewReader(distTest.Archive))
	if err != nil {
		return nil, err
	}

	registry := metrics.NewRegistry()
	piState := &lib.TestPreInitState{
		Logger: gs.Logger,
		RuntimeOptions: lib.RuntimeOptions{
			NoThresholds:      null.BoolFrom(true),
			NoSummary:         null.BoolFrom(true),
			Env:               arc.Env,
			CompatibilityMode: null.StringFrom(arc.CompatibilityMode),
		},
		Registry:       registry,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
	}

	initRunner, err := js.NewFromArchive(piState, arc)
	if err != nil {
		return nil, err
	}

	test := &loadedTest{
		pwd:            arc.Pwd,
		sourceRootPath: arc.Filename,
		source: &loader.SourceData{
			Data: distTest.Archive,
			URL:  arc.FilenameURL,
		},
		fs:           afero.NewMemMapFs(), // TODO: figure out what should be here
		fileSystems:  arc.Filesystems,
		preInitState: piState,
		initRunner:   initRunner,
	}

	pseudoConsoldatedConfig := applyDefault(Config{Options: options})
	for _, thresholds := range pseudoConsoldatedConfig.Thresholds {
		if err = thresholds.Parse(); err != nil {
			return nil, err
		}
	}
	derivedConfig, err := deriveAndValidateConfig(pseudoConsoldatedConfig, initRunner.IsExecutable, gs.Logger)
	if err != nil {
		return nil, err
	}

	return &loadedAndConfiguredTest{
		loadedTest:         test,
		consolidatedConfig: pseudoConsoldatedConfig,
		derivedConfig:      derivedConfig,
	}, nil
}

// TODO: a whole lot of cleanup, refactoring, error handling and hardening
func getCmdAgent(gs *state.GlobalState) *cobra.Command {
	c := &cmdsRunAndAgent{gs: gs}

	c.loadConfiguredTests = func(cmd *cobra.Command, args []string) (
		[]*loadedAndConfiguredTest, execution.Controller, error,
	) {
		// TODO: add some gRPC authentication
		conn, err := grpc.Dial(args[0], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}
		c.testEndHook = func(err error) {
			gs.Logger.Debugf("k6 agent run ended with err=%s", err)
			_ = conn.Close()
		}

		client := distributed.NewDistributedTestClient(conn)

		resp, err := client.Register(gs.Ctx, &distributed.RegisterRequest{})
		if err != nil {
			return nil, nil, err
		}

		var configuredTests []*loadedAndConfiguredTest
		for _, test := range resp.Tests {
			cTest, lcErr := loadAndConfigureDistTest(gs, test)
			if lcErr != nil {
				return nil, nil, lcErr
			}
			configuredTests = append(configuredTests, cTest)
		}

		c.metricsEngineHook = getMetricsHook(gs.Ctx, resp.InstanceID, client, gs.Logger)

		controller, err := distributed.NewAgentController(gs.Ctx, resp.InstanceID, client, gs.Logger)
		if err != nil {
			return nil, nil, err
		}

		gs.Flags.Address = "" // TODO: fix, this is a hack so agents don't start an API server

		return configuredTests, controller, nil // TODO
	}

	agentCmd := &cobra.Command{
		Use:    "agent",
		Short:  "Join a distributed load test",
		Long:   `TODO`,
		Args:   exactArgsWithMsg(1, "arg should either the IP and port of the controller k6 instance"),
		RunE:   c.run,
		Hidden: true, // TODO: remove when officially released
	}

	// TODO: add flags

	return agentCmd
}
