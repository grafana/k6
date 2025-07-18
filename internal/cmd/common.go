package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/types"

	// Blank-importing golang.org/x/crypto/x509roots/fallback bundles a set of
	// root fallback certificates from Mozilla into the resulting binary. This
	// allows the program to run in environments where the system root
	// certificates are not available, for example inside a minimal container.
	// These are _fallbacks_, meaning that if the system _does have_ a set of
	// root certificates, those will be given priority.
	_ "golang.org/x/crypto/x509roots/fallback"
)

// Panic if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: refactor the CLI config so these functions aren't needed - they
// can mask errors by failing only at runtime, not at compile time
func getNullBool(flags *pflag.FlagSet, key string) null.Bool {
	v, err := flags.GetBool(key)
	if err != nil {
		panic(err)
	}
	return null.NewBool(v, flags.Changed(key))
}

func getNullInt64(flags *pflag.FlagSet, key string) null.Int {
	v, err := flags.GetInt64(key)
	if err != nil {
		panic(err)
	}
	return null.NewInt(v, flags.Changed(key))
}

func getNullDuration(flags *pflag.FlagSet, key string) types.NullDuration {
	// TODO: use types.ParseExtendedDuration? not sure we should support
	// unitless durations (i.e. milliseconds) here...
	v, err := flags.GetDuration(key)
	if err != nil {
		panic(err)
	}
	return types.NullDuration{Duration: types.Duration(v), Valid: flags.Changed(key)}
}

func getNullString(flags *pflag.FlagSet, key string) null.String {
	v, err := flags.GetString(key)
	if err != nil {
		panic(err)
	}
	return null.NewString(v, flags.Changed(key))
}

func exactArgsWithMsg(n int, msg string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d: %s", n, len(args), msg)
		}
		return nil
	}
}

func printToStdout(gs *state.GlobalState, s string) {
	if _, err := fmt.Fprint(gs.Stdout, s); err != nil {
		gs.Logger.Errorf("could not print '%s' to stdout: %s", s, err.Error())
	}
}

func getExampleText(gs *state.GlobalState, tpl string) string {
	var exampleText bytes.Buffer
	exampleTemplate := template.Must(template.New("").Parse(tpl))

	if err := exampleTemplate.Execute(&exampleText, gs.BinaryName); err != nil {
		gs.Logger.WithError(err).Error("Error during help example generation")
	}

	return exampleText.String()
}

// Trap Interrupts, SIGINTs and SIGTERMs and call the given.
func handleTestAbortSignals(gs *state.GlobalState, gracefulStopHandler, onHardStop func(os.Signal)) (stop func()) {
	gs.Logger.Debug("Trapping interrupt signals so k6 can handle them gracefully...")
	sigC := make(chan os.Signal, 2)
	done := make(chan struct{})
	gs.SignalNotify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigC:
			gracefulStopHandler(sig)
		case <-done:
			return
		}

		select {
		case sig := <-sigC:
			if onHardStop != nil {
				onHardStop(sig)
			}
			// If we get a second signal, we immediately exit, so something like
			// https://github.com/k6io/k6/issues/971 never happens again
			gs.OSExit(int(exitcodes.ExternalAbort))
		case <-done:
			return
		}
	}()

	return func() {
		gs.Logger.Debug("Releasing signal trap...")
		close(done)
		gs.SignalStop(sigC)
	}
}

func resolveStackSlugToID(gs *state.GlobalState, jsonRawConf json.RawMessage, token, slug string) (int64, error) {
	slug = stripGrafanaNetSuffix(slug)
	stacks, err := getStacks(gs, jsonRawConf, token)
	if err != nil {
		return 0, err
	}
	id, ok := stacks[slug]
	if !ok {
		return 0, fmt.Errorf("stack slug %q not found in your Grafana Cloud account", slug)
	}
	return int64(id), nil
}

func stripGrafanaNetSuffix(s string) string {
	const suffix = ".grafana.net"
	if len(s) > len(suffix) && s[len(s)-len(suffix):] == suffix {
		return s[:len(s)-len(suffix)]
	}
	return s
}

func resolveDefaultProjectID(
	gs *state.GlobalState,
	apiClient *cloudapi.Client,
	jsonRawConf json.RawMessage,
	token string,
	stackSlug null.String,
	stackID *null.Int,
	projectID *int64,
) (int64, error) {
	if *projectID != 0 {
		return *projectID, nil
	}

	if stackID.Int64 == 0 {
		if stackSlug.Valid && stackSlug.String != "" {
			id, err := resolveStackSlugToID(gs, jsonRawConf, token, stackSlug.String)
			if err != nil {
				return 0, fmt.Errorf("could not resolve stack slug %q to stack ID: %w", stackSlug.String, err)
			}
			*stackID = null.IntFrom(id)
		} else {
			gs.Logger.Error("please specify a projectID in your test or use `k6 cloud login` to set up a default stack")
			return 0, fmt.Errorf("no projectID specified and no default stack set")
		}
	}

	pid, _, err := apiClient.GetDefaultProject(stackID.Int64)
	if err != nil {
		return 0, fmt.Errorf("can't get default projectID for stack %d (%s): %w", stackID.Int64, stackSlug.String, err)
	}
	*projectID = pid

	gs.Logger.Warnf("Warning: no projectID specified, using default project of the stack: %s \n\n", stackSlug.String)
	return pid, nil
}
