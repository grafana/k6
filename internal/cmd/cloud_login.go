package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/ui"
)

const cloudLoginCommandName = "login"

type cmdCloudLogin struct {
	globalState *state.GlobalState
}

func getCmdCloudLogin(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudLogin{
		globalState: gs,
	}

	// loginCloudCommand represents the 'cloud login' command
	exampleText := getExampleText(gs, `
  # Prompt for a Grafana Cloud k6 token
  $ {{.}} cloud login

  # Store a token in k6's persistent configuration
  $ {{.}} cloud login -t <YOUR_TOKEN>

  # Display the stored token
  $ {{.}} cloud login -s

  # Reset the stored token
  $ {{.}} cloud login -r`[1:])

	loginCloudCommand := &cobra.Command{
		Use:   cloudLoginCommandName,
		Short: "Authenticate with Grafana Cloud k6",
		Long: `Authenticate with Grafana Cloud k6.

This command will authenticate you with Grafana Cloud k6.
Once authenticated you can start running tests in the cloud by using the "k6 cloud run"
command, or by executing a test locally and outputting samples to the cloud using
the "k6 run -o cloud" command.
`,
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset stored token and stack info")
	loginCloudCommand.Flags().String("stack-id", "", "set stack ID (cannot be used with --stack-slug)")
	loginCloudCommand.Flags().String("stack-slug", "", "set stack slug (cannot be used with --stack-id)")

	return loginCloudCommand
}

// run is the code that runs when the user executes `k6 cloud login`
func (c *cmdCloudLogin) run(cmd *cobra.Command, _ []string) error {
	err := migrateLegacyConfigFileIfAny(c.globalState)
	if err != nil {
		return err
	}

	currentDiskConf, err := readDiskConfig(c.globalState)
	if err != nil {
		return err
	}

	currentJSONConfig := cloudapi.Config{}
	currentJSONConfigRaw := currentDiskConf.Collectors["cloud"]
	if currentJSONConfigRaw != nil {
		// We only want to modify this config, see comment below
		if jsonerr := json.Unmarshal(currentJSONConfigRaw, &currentJSONConfig); jsonerr != nil {
			return jsonerr
		}
	}

	// But we don't want to save them back to the JSON file, we only
	// want to save what already existed there and the login details.
	newCloudConf := currentJSONConfig

	show := getNullBool(cmd.Flags(), "show")
	reset := getNullBool(cmd.Flags(), "reset")
	token := getNullString(cmd.Flags(), "token")
	stackIDStr := cmd.Flags().Lookup("stack-id").Value.String()
	stackSlug := getNullString(cmd.Flags(), "stack-slug")

	var stackIDInt int64
	var stackIDValid bool
	if stackIDStr != "" {
		var parseErr error
		stackIDInt, parseErr = parseStackID(stackIDStr)
		if parseErr != nil {
			return fmt.Errorf("invalid --stack-id: %w", parseErr)
		}
		stackIDValid = true
	}
	if stackIDValid && stackSlug.Valid {
		return errors.New("only one of --stack-id or --stack-slug can be specified")
	}

	switch {
	case reset.Valid:
		newCloudConf.Token = null.StringFromPtr(nil)
		newCloudConf.StackID = null.StringFromPtr(nil)
		newCloudConf.StackSlug = null.StringFromPtr(nil)
		printToStdout(c.globalState, "  token and stack info reset\n")
		return nil
	case show.Bool:
		valueColor := getColor(c.globalState.Flags.NoColor || !c.globalState.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))
		if newCloudConf.StackID.Valid {
			printToStdout(c.globalState, fmt.Sprintf("  stack-id: %s\n", valueColor.Sprint(newCloudConf.StackID.String)))
		}
		if newCloudConf.StackSlug.Valid {
			printToStdout(c.globalState, fmt.Sprintf("  stack-slug: %s\n", valueColor.Sprint(newCloudConf.StackSlug.String)))
		}
		return nil
	case token.Valid:
		newCloudConf.Token = token
	default:
		tokenForm := ui.Form{
			Banner: "Enter your token to authenticate with Grafana Cloud k6.\n" +
				"Please, consult the Grafana Cloud k6 documentation for instructions on how to generate one:\n" +
				"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication",
			Fields: []ui.Field{
				ui.StringField{
					Key:   "Token",
					Label: "Token",
				},
			},
		}
		if !term.IsTerminal(int(syscall.Stdin)) { //nolint:unconvert
			c.globalState.Logger.Warn("Stdin is not a terminal, falling back to plain text input")
		}
		tokenVals, err := tokenForm.Run(c.globalState.Stdin, c.globalState.Stdout)
		if err != nil {
			return err
		}
		tokenValue := tokenVals["Token"]
		newCloudConf.Token = null.StringFrom(tokenValue)

		if newCloudConf.Token.Valid {
			if err := validateToken(c.globalState, currentJSONConfigRaw, newCloudConf.Token.String); err != nil {
				return fmt.Errorf("token validation failed: %w", err)
			}
		}

		defaultStack, err := func(token string) (string, error) {
			stacks, err := getStacks(c.globalState, currentJSONConfigRaw, token)
			if err != nil {
				return "", fmt.Errorf("failed to get default stack slug: %w", err)
			}

			// TODO: Can we make this better? Picking the first one is not ideal.
			for slug := range stacks {
				return slug, nil
			}
			return "", errors.New("no stacks found for the provided token, please create a stack in Grafana Cloud and initialize GCk6 app")
		}(tokenValue)
		if err != nil {
			return fmt.Errorf("failed to get default stack: %w", err)
		}

		stackForm := ui.Form{
			Banner: "\nConfigure the stack where your tests will run by default.\n" +
				"Please, use the slug from your Grafana Cloud URL, e.g. my-team from https://my-team.grafana.net.\n",
			Fields: []ui.Field{
				ui.StringField{
					Key:     "Stack",
					Label:   "Stack",
					Default: defaultStack,
				},
			},
		}
		stackVals, err := stackForm.Run(c.globalState.Stdin, c.globalState.Stdout)
		if err != nil {
			return err
		}

		stackValue := null.StringFrom(stackVals["Stack"])
		if stackValue.Valid {
			stackSlug = stackValue
		} else {
			return errors.New("stack cannot be empty")
		}
	}

	if stackIDValid {
		newCloudConf.StackID = null.StringFrom(fmt.Sprintf("%d", stackIDInt))
		newCloudConf.StackSlug = null.StringFromPtr(nil)
	} else if stackSlug.Valid {
		newCloudConf.StackSlug = stackSlug
		newCloudConf.StackID = null.StringFromPtr(nil)
	} else {
		newCloudConf.StackID = null.StringFromPtr(nil)
		newCloudConf.StackSlug = null.StringFromPtr(nil)
	}

	if newCloudConf.Token.Valid {
		err := validateToken(c.globalState, currentJSONConfigRaw, newCloudConf.Token.String)
		if err != nil {
			return err
		}
	}

	if currentDiskConf.Collectors == nil {
		currentDiskConf.Collectors = make(map[string]json.RawMessage)
	}
	currentDiskConf.Collectors["cloud"], err = json.Marshal(newCloudConf)
	if err != nil {
		return err
	}
	if err := writeDiskConfig(c.globalState, currentDiskConf); err != nil {
		return err
	}

	if newCloudConf.Token.Valid {
		valueColor := getColor(c.globalState.Flags.NoColor || !c.globalState.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.globalState, fmt.Sprintf(
			"\nLogged in successfully, token and stack info saved in %s\n", c.globalState.Flags.ConfigFilePath,
		))
		if !c.globalState.Flags.Quiet {
			printToStdout(c.globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))

			if newCloudConf.StackID.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-id: %s\n", valueColor.Sprint(newCloudConf.StackID.String)))
			}
			if newCloudConf.StackSlug.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-slug: %s\n", valueColor.Sprint(newCloudConf.StackSlug.String)))
			}
		}
	}
	return nil
}

func validateToken(gs *state.GlobalState, jsonRawConf json.RawMessage, token string) error {
	// We want to use this fully consolidated config for things like
	// host addresses, so users can overwrite them with env vars.
	consolidatedCurrentConfig, warn, err := cloudapi.GetConsolidatedConfig(
		jsonRawConf, gs.Env, "", nil, nil)
	if err != nil {
		return err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}

	client := cloudapi.NewClient(
		gs.Logger,
		token,
		consolidatedCurrentConfig.Host.String,
		build.Version,
		consolidatedCurrentConfig.Timeout.TimeDuration(),
	)

	var res *cloudapi.ValidateTokenResponse
	res, err = client.ValidateToken()
	if err != nil {
		return fmt.Errorf("can't validate the API token: %s", err.Error())
	}

	if !res.IsValid {
		return errors.New("your API token is invalid - " +
			"please, consult the Grafana Cloud k6 documentation for instructions on how to generate a new one:\n" +
			"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication")
	}

	return nil
}

func getStacks(gs *state.GlobalState, jsonRawConf json.RawMessage, token string) (map[string]int, error) {
	// We want to use this fully consolidated config for things like
	// host addresses, so users can overwrite them with env vars.
	consolidatedCurrentConfig, warn, err := cloudapi.GetConsolidatedConfig(
		jsonRawConf, gs.Env, "", nil, nil)
	if err != nil {
		return nil, err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}
	client := cloudapi.NewClient(
		gs.Logger,
		token,
		"",
		build.Version,
		consolidatedCurrentConfig.Timeout.TimeDuration(),
	)

	res, err := client.AccountMe()
	if err != nil {
		return nil, fmt.Errorf("can't get account info: %s", err.Error())
	}

	stacks := make(map[string]int)
	for _, organization := range res.Organizations {
		stacks[organization.GrafanaStackName] = organization.GrafanaStackID
	}

	return stacks, nil
}

func parseStackID(val string) (int64, error) {
	return strconv.ParseInt(val, 10, 64)
}
