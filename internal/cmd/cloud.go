package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/lib"

	"github.com/spf13/cobra"
)

var (
	errCloudAuth = errors.New( //nolint:staticcheck // user-facing error message, capitalization is intentional
		"Run `k6 cloud login` to authenticate, or check the docs for other options at" +
			" https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication",
	)
	errMissingToken   = errors.New("access token not configured")
	errMissingStackID = errors.New("stack ID not configured")
)

// checkCloudLogin verifies that both a token and a stack are configured.
// Together they represent a complete Grafana Cloud login.
func checkCloudLogin(conf cloudapi.Config) error {
	const prefix = "Running cloud tests requires auth settings"
	if !conf.Token.Valid || conf.Token.String == "" {
		return fmt.Errorf("%s: %w.\n%w", prefix, errMissingToken, errCloudAuth)
	}
	if !conf.StackID.Valid || conf.StackID.Int64 == 0 {
		return fmt.Errorf("%s: %w.\n%w", prefix, errMissingStackID, errCloudAuth)
	}
	return nil
}

func getCloudUsageTemplate() string {
	return `{{.Short}}

Usage:{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{else if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if .IsAvailableCommand}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}

Flags:
{{.LocalFlags.FlagUsagesWrapped 120 | trimTrailingWhitespaces}}
{{if .HasExample}}
Examples:
{{.Example}}
{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

func getCmdCloud(gs *state.GlobalState) *cobra.Command {
	exampleText := getExampleText(gs, `
  # Authenticate with Grafana Cloud
  $ {{.}} cloud login

  # Run a test script in Grafana Cloud
  $ {{.}} cloud run script.js

  # Run a test script locally and stream results to Grafana Cloud
  $ {{.}} cloud run --local-execution script.js

  # Run a test archive in Grafana Cloud
  $ {{.}} cloud run archive.tar`[1:])

	cloudCmd := &cobra.Command{
		Use:     "cloud",
		Short:   "Run and manage Grafana Cloud tests",
		Long:    "Run and manage tests in Grafana Cloud.",
		Example: exampleText,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Register `k6 cloud` subcommands with default usage template
	defaultUsageTemplate := (&cobra.Command{}).UsageTemplate()
	defaultUsageTemplate = strings.ReplaceAll(defaultUsageTemplate, "FlagUsages", "FlagUsagesWrapped 120")

	runCmd := getCmdCloudRun(gs)
	runCmd.SetUsageTemplate(defaultUsageTemplate)
	runCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(runCmd)

	loginCmd := getCmdCloudLogin(gs)
	loginCmd.SetUsageTemplate(defaultUsageTemplate)
	loginCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(loginCmd)

	uploadCmd := getCmdCloudUpload(gs)
	uploadCmd.SetUsageTemplate(defaultUsageTemplate)
	uploadCmd.SetHelpTemplate((&cobra.Command{}).HelpTemplate())
	cloudCmd.AddCommand(uploadCmd)

	projectCmd := getCmdCloudProject(gs)
	cloudCmd.AddCommand(projectCmd)

	cloudCmd.Flags().SortFlags = false

	// Use custom template similar to root - hardcode flags to avoid showing global flags
	cloudTemplate := getCloudUsageTemplate()
	cloudCmd.SetUsageTemplate(cloudTemplate)
	cloudCmd.SetHelpTemplate(cloudTemplate)

	return cloudCmd
}

// prepCloudTestRun wires stack and project IDs into the client, validates
// script options, and resolves a default project when none was specified.
// Returns the project ID as used by subsequent v6 calls.
func prepCloudTestRun(
	ctx context.Context, gs *state.GlobalState,
	client *cloudapiv6.Client,
	cloudConfig *cloudapi.Config, tmpCloudConfig map[string]any, arc *lib.Archive,
) (int32, error) {
	toInt32 := func(v int64) (int32, error) {
		if v < math.MinInt32 || v > math.MaxInt32 {
			return 0, fmt.Errorf("value %d overflows int32", v)
		}
		return int32(v), nil
	}

	stackID, err := toInt32(cloudConfig.StackID.Int64)
	if err != nil {
		return 0, err
	}
	client.SetStackID(stackID)

	projectID, err := toInt32(cloudConfig.ProjectID.Int64)
	if err != nil {
		return 0, err
	}

	if projectID == 0 {
		if err := resolveAndSetProjectID(gs, cloudConfig, tmpCloudConfig, arc); err != nil {
			return 0, err
		}
		projectID, err = toInt32(cloudConfig.ProjectID.Int64)
		if err != nil {
			return 0, err
		}
	}

	if err := client.ValidateOptions(ctx, projectID, arc.Options); err != nil {
		return 0, err
	}

	return projectID, nil
}

func resolveDefaultProjectID(
	gs *state.GlobalState,
	cloudConfig *cloudapi.Config,
) (int64, error) {
	// Priority: projectID -> default stack from config
	if cloudConfig.ProjectID.Valid && cloudConfig.ProjectID.Int64 > 0 {
		return cloudConfig.ProjectID.Int64, nil
	}
	if cloudConfig.StackID.Valid && cloudConfig.StackID.Int64 != 0 {
		if cloudConfig.DefaultProjectID.Valid && cloudConfig.DefaultProjectID.Int64 > 0 {
			stackName := cloudConfig.StackURL.String
			if !cloudConfig.StackURL.Valid {
				stackName = fmt.Sprintf("stack-%d", cloudConfig.StackID.Int64)
			}
			gs.Logger.Warnf("No projectID specified, using default project of the %s stack\n", stackName)
			return cloudConfig.DefaultProjectID.Int64, nil
		}
		return 0, fmt.Errorf(
			"default stack configured but the default project ID is not available - " +
				"please run `k6 cloud login` to refresh your configuration")
	}

	// Return 0 to let the backend pick the project (old behavior)
	return 0, nil
}

func resolveAndSetProjectID(
	gs *state.GlobalState,
	cloudConfig *cloudapi.Config,
	tmpCloudConfig map[string]any,
	arc *lib.Archive,
) error {
	projectID, err := resolveDefaultProjectID(gs, cloudConfig)
	if err != nil {
		return err
	}
	if projectID > 0 {
		tmpCloudConfig["projectID"] = projectID

		b, err := json.Marshal(tmpCloudConfig)
		if err != nil {
			return err
		}

		arc.Options.Cloud = b

		cloudConfig.ProjectID = null.IntFrom(projectID)
	}
	return nil
}
