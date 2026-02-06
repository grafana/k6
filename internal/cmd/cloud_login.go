package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	v6cloudapi "go.k6.io/k6/internal/cloudapi/v6"
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
  # Authenticate interactively with Grafana Cloud
  $ {{.}} cloud login

  # Store a token in k6's persistent configuration
  $ {{.}} cloud login -t <YOUR_TOKEN>

  # Store a token in k6's persistent configuration and set the stack
  $ {{.}} cloud login -t <YOUR_TOKEN> --stack <YOUR_STACK_URL_OR_SLUG>

  # Display the stored token and stack info
  $ {{.}} cloud login -s

  # Reset the stored token and stack info
  $ {{.}} cloud login -r`[1:])

	loginCloudCommand := &cobra.Command{
		Use:     cloudLoginCommandName,
		Short:   "Authenticate with Grafana Cloud",
		Long:    "Authenticate with Grafana Cloud. Required before running cloud tests.",
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token, stack info and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset stored token and stack info")
	loginCloudCommand.Flags().String("stack", "", "specify the stack (URL or slug) where commands will run by default")

	return loginCloudCommand
}

// run is the code that runs when the user executes `k6 cloud login`
//
//nolint:funlen
func (c *cmdCloudLogin) run(cmd *cobra.Command, _ []string) error {
	if !checkIfMigrationCompleted(c.globalState) {
		err := migrateLegacyConfigFileIfAny(c.globalState)
		if err != nil {
			return err
		}
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
	stackInput := getNullString(cmd.Flags(), "stack")

	switch {
	case reset.Valid:
		newCloudConf.Token = null.StringFromPtr(nil)
		newCloudConf.StackID = null.IntFromPtr(nil)
		newCloudConf.StackURL = null.StringFromPtr(nil)
		newCloudConf.DefaultProjectID = null.IntFromPtr(nil)
		printToStdout(c.globalState, "  token and stack info reset\n")
	case show.Bool:
		printConfig(c.globalState, newCloudConf)
		return nil
	case token.Valid:
		err := validateInputs(c.globalState, &newCloudConf, currentJSONConfigRaw, token, stackInput)
		if err != nil {
			return err
		}
	default:
		gs := c.globalState

		/* Token form */
		tokenForm := ui.Form{
			Banner: "Enter your token to authenticate with Grafana Cloud.\n" +
				"Please, consult the documentation for instructions on how to generate one:\n" +
				"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication",
			Fields: []ui.Field{
				ui.PasswordField{
					Key:   "Token",
					Label: "Token",
				},
			},
		}
		if !term.IsTerminal(int(syscall.Stdin)) { //nolint:unconvert
			gs.Logger.Warn("Stdin is not a terminal, falling back to plain text input")
		}
		tokenVals, err := tokenForm.Run(gs.Stdin, gs.Stdout)
		if err != nil {
			return err
		}
		tokenInput := null.StringFrom(tokenVals["Token"])
		if !tokenInput.Valid {
			return errors.New("token cannot be empty")
		}

		/* Stack form */
		stackForm := ui.Form{
			Banner: "\nEnter the stack where you want to run k6's commands by default.\n" +
				"You can enter a full URL (e.g. https://my-team.grafana.net) or just the slug (e.g. my-team):",
			Fields: []ui.Field{
				ui.StringField{
					Key:     "Stack",
					Label:   "Stack",
					Default: "None",
				},
			},
		}
		stackVals, err := stackForm.Run(gs.Stdin, gs.Stdout)
		if err != nil {
			return err
		}
		stackInput := null.StringFrom(strings.TrimSpace(stackVals["Stack"]))

		err = validateInputs(gs, &newCloudConf, currentJSONConfigRaw, tokenInput, stackInput)
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

	if !newCloudConf.Token.Valid {
		return nil
	}

	printToStdout(c.globalState, fmt.Sprintf(
		"\nLogged in successfully, token and stack info saved in %s\n", c.globalState.Flags.ConfigFilePath,
	))
	if !c.globalState.Flags.Quiet {
		printConfig(c.globalState, newCloudConf)
	}

	return nil
}

func printConfig(gs *state.GlobalState, cloudConf cloudapi.Config) {
	valueColor := getColor(gs.Flags.NoColor || !gs.Stdout.IsTTY, color.FgCyan)
	printToStdout(gs, fmt.Sprintf("  token: %s\n", valueColor.Sprint(cloudConf.Token.String)))

	if !cloudConf.StackID.Valid && !cloudConf.StackURL.Valid {
		printToStdout(gs, "  stack-id: <not set>\n")
		printToStdout(gs, "  stack-url: <not set>\n")
		printToStdout(gs, "  default-project-id: <not set>\n")

		return
	}

	if cloudConf.StackID.Valid {
		printToStdout(gs, fmt.Sprintf("  stack-id: %s\n", valueColor.Sprint(cloudConf.StackID.Int64)))
	}
	if cloudConf.StackURL.Valid {
		printToStdout(gs, fmt.Sprintf("  stack-url: %s\n", valueColor.Sprint(cloudConf.StackURL.String)))
	}
	if cloudConf.DefaultProjectID.Valid {
		printToStdout(gs, fmt.Sprintf("  default-project-id: %s\n",
			valueColor.Sprint(cloudConf.DefaultProjectID.Int64)))
	}
}

// validateInputs validates a token and a stack if provided
// and update the config with the given inputs
func validateInputs(
	gs *state.GlobalState,
	config *cloudapi.Config,
	rawConfig json.RawMessage,
	token, stackInput null.String,
) error {
	config.Token = token
	consolidatedCurrentConfig, warn, err := cloudapi.GetConsolidatedConfig(
		rawConfig, gs.Env, "", nil, nil)
	if err != nil {
		return err
	}
	if warn != "" {
		gs.Logger.Warn(warn)
	}

	stackValue := stackInput.String
	if stackInput.Valid && stackValue != "" && stackValue != "None" {
		stackURL, stackID, defaultProjectID, err := validateTokenV6(
			gs, consolidatedCurrentConfig, token.String, stackValue)
		if err != nil {
			return fmt.Errorf(
				"your stack is invalid - please, consult the documentation "+
					"for instructions on how to get yours: "+
					"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/configure-stack. "+
					"Error details: %w",
				err)
		}
		config.StackURL = null.StringFrom(stackURL)
		config.StackID = null.IntFrom(stackID)
		config.DefaultProjectID = null.IntFrom(defaultProjectID)
	} else {
		err = validateTokenV1(gs, consolidatedCurrentConfig, config.Token.String)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateTokenV1 validates a token using v1 cloud API.
//
// Deprecated: use validateTokenV6 instead if a stack name is provided.
func validateTokenV1(gs *state.GlobalState, config cloudapi.Config, token string) error {
	client := cloudapi.NewClient(
		gs.Logger,
		token,
		config.Host.String,
		build.Version,
		config.Timeout.TimeDuration(),
	)

	res, err := client.ValidateToken()
	if err != nil {
		return fmt.Errorf("can't validate the API token: %s", err.Error())
	}

	if !res.IsValid {
		return errors.New("your API token is invalid - " +
			"please, consult the documentation for instructions on how to generate a new one:\n" +
			"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication")
	}

	return nil
}

// validateTokenV6 validates a token and a stack URL/slug and returns the normalized URL, stack ID,
// and default project ID.
// The stackInput can be either a full URL (e.g., https://my-team.grafana.net) or just a slug (e.g., my-team).
func validateTokenV6(
	gs *state.GlobalState,
	config cloudapi.Config,
	token, stackInput string,
) (stackURL string, stackID int64, defaultProjectID int64, err error) {
	normalizedURL := normalizeStackURL(stackInput)

	client, err := v6cloudapi.NewClient(
		gs.Logger,
		token,
		config.Hostv6.String,
		build.Version,
		config.Timeout.TimeDuration(),
	)
	if err != nil {
		return "", 0, 0, err
	}

	authResp, err := client.ValidateToken(normalizedURL)
	if err != nil {
		return "", 0, 0, err
	}

	return normalizedURL, int64(authResp.StackId), int64(authResp.DefaultProjectId), nil
}

// normalizeStackURL converts a stack slug to a full URL if needed.
// The stackInput can be either a full URL (e.g., https://my-team.grafana.net) or just a slug (e.g., my-team).
func normalizeStackURL(stackInput string) string {
	// If it's already a full URL, return it as is
	if strings.HasPrefix(stackInput, "http://") || strings.HasPrefix(stackInput, "https://") {
		return stackInput
	}
	// Otherwise, treat it as a slug and construct the URL
	slug := stripGrafanaNetSuffix(stackInput)
	return fmt.Sprintf("https://%s.grafana.net", slug)
}

// stripGrafanaNetSuffix removes .grafana.net suffix if present.
func stripGrafanaNetSuffix(s string) string {
	const suffix = ".grafana.net"
	if len(s) > len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}
