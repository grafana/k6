package cmd

import (
	"fmt"

	"go.k6.io/k6/errext/exitcodes"

	"go.k6.io/k6/errext"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.k6.io/k6/internal/execution"
	"go.k6.io/k6/internal/execution/local"
)

const cloudRunCommandName string = "run"

type cmdCloudRun struct {
	// localExecution stores the state of the --local-execution flag.
	localExecution bool

	// linger stores the state of the --linger flag.
	linger bool

	// noUsageReport stores the state of the --no-usage-report flag.
	noUsageReport bool

	// noArchiveUpload stores the state of the --no-archive-upload flag.
	//
	// This flag indicates to the local execution mode to not send the test
	// archive to the cloud service.
	noArchiveUpload bool

	// runCmd holds an instance of the k6 run command that we store
	// in order to be able to call its run method to support
	// the --local-execution flag mode.
	runCmd *cmdRun

	// deprecatedCloudCmd holds an instance of the k6 cloud command that we store
	// in order to be able to call its run method to support the cloud execution
	// feature, and to have access to its flagSet if necessary.
	deprecatedCloudCmd *cmdCloud
}

func getCmdCloudRun(cloudCmd *cmdCloud) *cobra.Command {
	// We instantiate the run command here to be able to call its run method
	// when the --local-execution flag is set.
	runCmd := &cmdRun{
		gs: cloudCmd.gs,

		// We override the loadConfiguredTest func to use the local execution
		// configuration which enforces the use of the cloud output among other
		// side effects.
		loadConfiguredTest: func(cmd *cobra.Command, args []string) (
			*loadedAndConfiguredTest,
			execution.Controller,
			error,
		) {
			test, err := loadAndConfigureLocalTest(cloudCmd.gs, cmd, args, getCloudRunLocalExecutionConfig)
			return test, local.NewController(), err
		},
	}

	cloudRunCmd := &cmdCloudRun{
		deprecatedCloudCmd: cloudCmd,
		runCmd:             runCmd,
	}

	exampleText := getExampleText(cloudCmd.gs, `
  # Run a test script in Grafana Cloud k6
  $ {{.}} cloud run script.js

  # Run a test archive in Grafana Cloud k6
  $ {{.}} cloud run archive.tar

  # Read a test script or archive from stdin and run it in Grafana Cloud k6
  $ {{.}} cloud run - < script.js`[1:])

	thisCmd := &cobra.Command{
		Use:   cloudRunCommandName,
		Short: "Run a test in Grafana Cloud k6",
		Long: `Run a test in Grafana Cloud k6.

This will archive test script(s), including all necessary resources, and execute the test in the Grafana Cloud k6
service. Using this command requires to be authenticated against Grafana Cloud k6.
Use the "k6 cloud login" command to authenticate.`,
		Example: exampleText,
		Args: exactArgsWithMsg(1,
			"the k6 cloud run command expects a single argument consisting in either a path to a script or "+
				"archive file, or the \"-\" symbol indicating the script or archive should be read from stdin",
		),
		PreRunE: cloudRunCmd.preRun,
		RunE:    cloudRunCmd.run,
	}

	thisCmd.Flags().SortFlags = false
	thisCmd.Flags().AddFlagSet(cloudRunCmd.flagSet())
	thisCmd.Flags().AddFlagSet(cloudCmd.flagSet())

	return thisCmd
}

func (c *cmdCloudRun) preRun(cmd *cobra.Command, args []string) error {
	if c.localExecution {
		if cmd.Flags().Changed("exit-on-running") {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("the --local-execution flag is not compatible with the --exit-on-running flag"),
				exitcodes.InvalidConfig,
			)
		}

		if cmd.Flags().Changed("show-logs") {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("the --local-execution flag is not compatible with the --show-logs flag"),
				exitcodes.InvalidConfig,
			)
		}

		return nil
	}

	if c.linger {
		return errext.WithExitCodeIfNone(
			fmt.Errorf("the --linger flag can only be used in conjunction with the --local-execution flag"),
			exitcodes.InvalidConfig,
		)
	}

	return c.deprecatedCloudCmd.preRun(cmd, args)
}

func (c *cmdCloudRun) run(cmd *cobra.Command, args []string) error {
	if c.localExecution {
		c.runCmd.loadConfiguredTest = func(*cobra.Command, []string) (*loadedAndConfiguredTest, execution.Controller, error) {
			test, err := loadAndConfigureLocalTest(c.runCmd.gs, cmd, args, getCloudRunLocalExecutionConfig)
			if err != nil {
				return nil, nil, fmt.Errorf("could not load and configure the test: %w", err)
			}

			if err := createCloudTest(c.runCmd.gs, test); err != nil {
				return nil, nil, fmt.Errorf("could not create the cloud test run: %w", err)
			}

			return test, local.NewController(), nil
		}
		return c.runCmd.run(cmd, args)
	}

	// When running the `k6 cloud run` command explicitly disable the usage report.
	c.noUsageReport = true

	return c.deprecatedCloudCmd.run(cmd, args)
}

func (c *cmdCloudRun) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false

	flags.BoolVar(&c.localExecution, "local-execution", c.localExecution,
		"executes the test locally instead of in the cloud")
	flags.BoolVar(
		&c.linger,
		"linger",
		c.linger,
		"only when using the local-execution mode, keeps the API server alive past the test end",
	)
	flags.BoolVar(
		&c.noUsageReport,
		"no-usage-report",
		c.noUsageReport,
		"only when using the local-execution mode, don't send anonymous usage "+
			"stats (https://grafana.com/docs/k6/latest/set-up/usage-collection/)",
	)
	flags.BoolVar(
		&c.noArchiveUpload,
		"no-archive-upload",
		c.noArchiveUpload,
		"only when using the local-execution mode, don't upload the test archive to the cloud service",
	)

	return flags
}

func getCloudRunLocalExecutionConfig(flags *pflag.FlagSet) (Config, error) {
	opts, err := getOptions(flags)
	if err != nil {
		return Config{}, err
	}

	// When running locally, we force the output to be cloud.
	out := []string{"cloud"}

	return Config{
		Options:         opts,
		Out:             out,
		Linger:          getNullBool(flags, "linger"),
		NoUsageReport:   getNullBool(flags, "no-usage-report"),
		NoArchiveUpload: getNullBool(flags, "no-archive-upload"),
	}, nil
}
