package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"go.k6.io/k6/cmd/state"
)

// cmdArchive handles the `k6 archive` sub-command
type cmdArchive struct {
	gs *state.GlobalState

	archiveOut     string
	excludeEnvVars bool
}

func (c *cmdArchive) run(cmd *cobra.Command, args []string) error {
	test, err := loadAndConfigureLocalTest(c.gs, cmd, args, getPartialConfig)
	if err != nil {
		return err
	}

	// It's important to NOT set the derived options back to the runner
	// here, only the consolidated ones. Otherwise, if the script used
	// an execution shortcut option (e.g. `iterations` or `duration`),
	// we will have multiple conflicting execution options since the
	// derivation will set `scenarios` as well.
	testRunState, err := test.buildTestRunState(test.consolidatedConfig.Options)
	if err != nil {
		return err
	}

	// Archive.
	arc := testRunState.Runner.MakeArchive()

	if c.excludeEnvVars {
		c.gs.Logger.Debug("environment variables will be excluded from the archive")

		arc.Env = nil
	}

	if c.archiveOut == "-" {
		return arc.Write(c.gs.Stdout)
	}

	f, err := c.gs.FS.Create(c.archiveOut)
	if err != nil {
		return err
	}

	err = arc.Write(f)
	if cerr := f.Close(); err == nil && cerr != nil {
		err = cerr
	}
	return err
}

func (c *cmdArchive) flagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.SortFlags = false
	flags.AddFlagSet(optionFlagSet())
	flags.AddFlagSet(runtimeOptionFlagSet(false))
	flags.StringVarP(
		&c.archiveOut, "archive-out", "O", c.archiveOut,
		"archive output filename. Dash (-) is a reserved value that causes the archive to be output to stdout.",
	)
	flags.BoolVarP(
		&c.excludeEnvVars,
		"exclude-env-vars",
		"",
		false,
		"do not embed any environment variables (either from --env or the actual environment) in the archive metadata",
	)

	return flags
}

func getCmdArchive(gs *state.GlobalState) *cobra.Command {
	c := &cmdArchive{
		gs:         gs,
		archiveOut: "archive.tar",
	}

	exampleText := getExampleText(gs, `
  # Archive a test run.
  {{.}} archive -u 10 -d 10s -O myarchive.tar script.js

  # Run the resulting archive.
  {{.}} run myarchive.tar`[1:])

	archiveCmd := &cobra.Command{
		Use:   "archive",
		Short: "Create an archive",
		Long: `Create an archive.

An archive is a fully self-contained test run, and can be executed identically elsewhere.`,
		Example: exampleText,
		Args:    cobra.ExactArgs(1),
		RunE:    c.run,
	}

	archiveCmd.Flags().SortFlags = false
	archiveCmd.Flags().AddFlagSet(c.flagSet())

	return archiveCmd
}
