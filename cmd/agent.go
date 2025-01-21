package cmd

import (
	"bytes"
	"encoding/json"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/execution"
	"go.k6.io/k6/execution/distributed"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/loader"
	"go.k6.io/k6/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/guregu/null.v3"
)

// TODO: a whole lot of cleanup, refactoring, error handling and hardening
func getCmdAgent(gs *state.GlobalState) *cobra.Command { //nolint: funlen
	c := &cmdsRunAndAgent{gs: gs}

	c.loadConfiguredTest = func(cmd *cobra.Command, args []string) (
		*loadedAndConfiguredTest, execution.Controller, error,
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

		controller, err := distributed.NewAgentController(gs.Ctx, resp.InstanceID, client, gs.Logger)
		if err != nil {
			return nil, nil, err
		}

		var options lib.Options
		if err = json.Unmarshal(resp.Options, &options); err != nil {
			return nil, nil, err
		}

		arc, err := lib.ReadArchive(bytes.NewReader(resp.Archive))
		if err != nil {
			return nil, nil, err
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
			return nil, nil, err
		}

		test := &loadedTest{
			pwd:            arc.Pwd,
			sourceRootPath: arc.Filename,
			source: &loader.SourceData{
				Data: resp.Archive,
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
				return nil, nil, err
			}
		}
		derivedConfig, err := deriveAndValidateConfig(pseudoConsoldatedConfig, initRunner.IsExecutable, gs.Logger)
		if err != nil {
			return nil, nil, err
		}

		configuredTest := &loadedAndConfiguredTest{
			loadedTest:         test,
			consolidatedConfig: pseudoConsoldatedConfig,
			derivedConfig:      derivedConfig,
		}

		gs.Flags.Address = "" // TODO: fix, this is a hack so agents don't start an API server

		return configuredTest, controller, nil // TODO
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
