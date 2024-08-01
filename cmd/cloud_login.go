package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/ui"
)

const cloudLoginCommandName = "login"

type cmdCloudLogin struct {
	globalState *state.GlobalState
}

const warning = `
[Warning]
Support for authentication with email and password will be removed after November 11th, 2024 (v0.55.0). 
Ensure you migrate to token-based authentication by then.

`

func getCmdCloudLogin(gs *state.GlobalState) *cobra.Command {
	c := &cmdCloudLogin{
		globalState: gs,
	}

	// loginCloudCommand represents the 'cloud login' command
	exampleText := getExampleText(gs, `
  # Log in with an email/password
  {{.}} cloud login

  # Store a token in k6's persistent configuration
  {{.}} cloud login -t <YOUR_TOKEN>

  # Display the stored token
  {{.}} cloud login -s
  
  # Reset the stored token
  {{.}} cloud login -r`[1:])

	loginCloudCommand := &cobra.Command{
		Use:   cloudLoginCommandName,
		Short: "Authenticate with Grafana Cloud k6",
		Long: fmt.Sprintf(`Authenticate with Grafana Cloud k6.

This command will authenticate you with Grafana Cloud k6.
Once authenticated you can start running tests in the cloud by using the "k6 cloud run"
command, or by executing a test locally and outputting samples to the cloud using
the "k6 run -o cloud" command.
%s
`, warning),
		Example: exampleText,
		Args:    cobra.NoArgs,
		RunE:    c.run,
	}

	loginCloudCommand.Flags().StringP("token", "t", "", "specify `token` to use")
	loginCloudCommand.Flags().BoolP("show", "s", false, "display saved token and exit")
	loginCloudCommand.Flags().BoolP("reset", "r", false, "reset stored token")

	return loginCloudCommand
}

// run is the code that runs when the user executes `k6 cloud login`
//
//nolint:funlen
func (c *cmdCloudLogin) run(cmd *cobra.Command, _ []string) error {
	warnColor := getColor(c.globalState.Flags.NoColor || !c.globalState.Stdout.IsTTY, color.FgYellow)
	printToStdout(c.globalState, warnColor.Sprint(warning))

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

	// We want to use this fully consolidated config for things like
	// host addresses, so users can overwrite them with env vars.
	consolidatedCurrentConfig, warn, err := cloudapi.GetConsolidatedConfig(
		currentJSONConfigRaw, c.globalState.Env, "", nil, nil)
	if err != nil {
		return err
	}

	if warn != "" {
		c.globalState.Logger.Warn(warn)
	}

	// But we don't want to save them back to the JSON file, we only
	// want to save what already existed there and the login details.
	newCloudConf := currentJSONConfig

	show := getNullBool(cmd.Flags(), "show")
	reset := getNullBool(cmd.Flags(), "reset")
	token := getNullString(cmd.Flags(), "token")
	switch {
	case reset.Valid:
		newCloudConf.Token = null.StringFromPtr(nil)
		printToStdout(c.globalState, "  token reset\n")
	case show.Bool:
	case token.Valid:
		newCloudConf.Token = token
	default:
		form := ui.Form{
			Banner: "Please enter your Grafana Cloud k6 credentials",
			Fields: []ui.Field{
				ui.StringField{
					Key:   "Email",
					Label: "Email",
				},
				ui.PasswordField{
					Key:   "Password",
					Label: "Password",
				},
			},
		}
		if !term.IsTerminal(int(syscall.Stdin)) { //nolint:unconvert
			c.globalState.Logger.Warn("Stdin is not a terminal, falling back to plain text input")
		}
		var vals map[string]string
		vals, err = form.Run(c.globalState.Stdin, c.globalState.Stdout)
		if err != nil {
			return err
		}
		email := vals["Email"]
		password := vals["Password"]

		client := cloudapi.NewClient(
			c.globalState.Logger,
			"",
			consolidatedCurrentConfig.Host.String,
			consts.Version,
			consolidatedCurrentConfig.Timeout.TimeDuration())

		var res *cloudapi.LoginResponse
		res, err = client.Login(email, password)
		if err != nil {
			return err
		}

		if res.Token == "" {
			return errors.New("your account does not appear to have an active API token, please consult the " +
				"Grafana Cloud k6 documentation for instructions on how to generate " +
				"one: https://grafana.com/docs/grafana-cloud/testing/k6/author-run/tokens-and-cli-authentication")
		}

		newCloudConf.Token = null.StringFrom(res.Token)
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
		if !c.globalState.Flags.Quiet {
			printToStdout(c.globalState, fmt.Sprintf("  token: %s\n", valueColor.Sprint(newCloudConf.Token.String)))
		}
		printToStdout(c.globalState, fmt.Sprintf(
			"Logged in successfully, token saved in %s\n", c.globalState.Flags.ConfigFilePath,
		))
	}
	return nil
}
