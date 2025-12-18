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

// validateAndResolveStack validates a stack URL/slug and returns the normalized URL, stack ID, and default project ID.
// The stackInput can be either a full URL (e.g., https://my-team.grafana.net) or just a slug (e.g., my-team).
func validateAndResolveStack(
	gs *state.GlobalState,
	jsonRawConf json.RawMessage,
	token, stackInput string,
) (stackURL string, stackID int64, defaultProjectID int64, err error) {
	consolidatedCurrentConfig, warn, err := cloudapi.GetConsolidatedConfig(
		jsonRawConf, gs.Env, "", nil, nil)
	if err != nil {
		return "", 0, 0, err
	}

	if warn != "" {
		gs.Logger.Warn(warn)
	}

	normalizedURL := normalizeStackURL(stackInput)

	client, err := v6cloudapi.NewClient(
		gs.Logger,
		token,
		consolidatedCurrentConfig.Host.String,
		build.Version,
		consolidatedCurrentConfig.Timeout.TimeDuration(),
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
  # Authenticate interactively with Grafana Cloud k6
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
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token, stack info and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset stored token and stack info")
	loginCloudCommand.Flags().String("stack", "", "specify the stack (URL or slug) where commands will run by default")

	return loginCloudCommand
}

// run is the code that runs when the user executes `k6 cloud login`
//
//nolint:funlen,gocognit,cyclop
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
		valueColor := getColor(c.globalState.Flags.NoColor || !c.globalState.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))
		if !newCloudConf.StackID.Valid && !newCloudConf.StackURL.Valid {
			printToStdout(c.globalState, "  stack-id: <not set>\n")
			printToStdout(c.globalState, "  stack-url: <not set>\n")
			printToStdout(c.globalState, "  default-project-id: <not set>\n")
		} else {
			if newCloudConf.StackID.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-id: %s\n", valueColor.Sprint(newCloudConf.StackID.Int64)))
			}
			if newCloudConf.StackURL.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-url: %s\n", valueColor.Sprint(newCloudConf.StackURL.String)))
			}
			if newCloudConf.DefaultProjectID.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  default-project-id: %s\n",
					valueColor.Sprint(newCloudConf.DefaultProjectID.Int64)))
			}
		}
		return nil
	case token.Valid:
		newCloudConf.Token = token

		err := validateToken(c.globalState, currentJSONConfigRaw, newCloudConf.Token.String)
		if err != nil {
			return err
		}

		if stackInput.Valid && stackInput.String != "" {
			stackURL, stackID, defaultProjectID, err := validateAndResolveStack(
				c.globalState, currentJSONConfigRaw, token.String, stackInput.String)
			if err != nil {
				return fmt.Errorf(
					"your stack is invalid - please, consult the Grafana Cloud k6 documentation "+
						"for instructions on how to get yours: "+
						"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/configure-stack. "+
						"Error details: %w",
					err)
			}
			newCloudConf.StackURL = null.StringFrom(stackURL)
			newCloudConf.StackID = null.IntFrom(stackID)
			newCloudConf.DefaultProjectID = null.IntFrom(defaultProjectID)
		}
	default:
		/* Token form */
		tokenForm := ui.Form{
			Banner: "Enter your token to authenticate with Grafana Cloud k6.\n" +
				"Please, consult the Grafana Cloud k6 documentation for instructions on how to generate one:\n" +
				"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication",
			Fields: []ui.Field{
				ui.PasswordField{
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

		if !newCloudConf.Token.Valid {
			return errors.New("token cannot be empty")
		}
		if err := validateToken(c.globalState, currentJSONConfigRaw, newCloudConf.Token.String); err != nil {
			return fmt.Errorf("token validation failed: %w", err)
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
		stackVals, err := stackForm.Run(c.globalState.Stdin, c.globalState.Stdout)
		if err != nil {
			return err
		}
		stackValue := strings.TrimSpace(stackVals["Stack"])
		if stackValue != "" && stackValue != "None" {
			stackURL, stackID, defaultProjectID, err := validateAndResolveStack(
				c.globalState, currentJSONConfigRaw, tokenValue, stackValue)
			if err != nil {
				return fmt.Errorf(
					"your stack is invalid - please, consult the Grafana Cloud k6 documentation "+
						"for instructions on how to get yours: "+
						"https://grafana.com/docs/grafana-cloud/testing/k6/author-run/configure-stack. "+
						"Error details: %w",
					err)
			}
			newCloudConf.StackURL = null.StringFrom(stackURL)
			newCloudConf.StackID = null.IntFrom(stackID)
			newCloudConf.DefaultProjectID = null.IntFrom(defaultProjectID)
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

	//nolint:nestif
	if newCloudConf.Token.Valid {
		valueColor := getColor(c.globalState.Flags.NoColor || !c.globalState.Stdout.IsTTY, color.FgCyan)
		printToStdout(c.globalState, fmt.Sprintf(
			"\nLogged in successfully, token and stack info saved in %s\n", c.globalState.Flags.ConfigFilePath,
		))
		if !c.globalState.Flags.Quiet {
			printToStdout(c.globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))

			if newCloudConf.StackID.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-id: %s\n", valueColor.Sprint(newCloudConf.StackID.Int64)))
			}
			if newCloudConf.StackURL.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  stack-url: %s\n", valueColor.Sprint(newCloudConf.StackURL.String)))
			}
			if newCloudConf.DefaultProjectID.Valid {
				printToStdout(c.globalState, fmt.Sprintf("  default-project-id: %s\n",
					valueColor.Sprint(newCloudConf.DefaultProjectID.Int64)))
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
