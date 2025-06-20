package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
)

var (
	errScriptNotFound     = errors.New("script not found")
	errUnsupportedFeature = errors.New("not supported")
)

// commandExecutor executes the requested k6 command line command.
// It abstract the execution path from the concrete binary.
type commandExecutor interface {
	run(*state.GlobalState)
}

// Launcher is a k6 launcher. It analyses the requirements of a k6 execution,
// then if required, it provisions a binary executor to satisfy the requirements.
type Launcher struct {
	// gs is the global state of k6.
	gs *state.GlobalState

	// provision generates a custom binary from the received list of dependencies
	// with their constrains, and it returns an executor that satisfies them.
	provision func(*state.GlobalState, k6deps.Dependencies) (commandExecutor, error)

	// commandExecutor executes the requested k6 command line command
	commandExecutor commandExecutor
}

// NewLauncher creates a new Launcher from a GlobalState using the default fallback and provision functions
func NewLauncher(gs *state.GlobalState) *Launcher {
	defaultExecutor := &currentBinary{}
	return &Launcher{
		gs:              gs,
		provision:       k6buildProvision,
		commandExecutor: defaultExecutor,
	}
}

// Launch executes k6 either by launching a provisioned binary or defaulting to the
// current binary if this is not necessary.
// The commandExecutor can exit the process so don't assume it will return
func (l *Launcher) Launch() {
	// If binary provisioning is not enabled, continue with the regular k6 execution path
	if !l.gs.Flags.BinaryProvisioning {
		l.gs.Logger.Debug("Binary Provisioning feature is disabled.")
		l.commandExecutor.run(l.gs)
		return
	}

	l.gs.Logger.
		Debug("Binary Provisioning feature is enabled.")
	deps, err := analyze(l.gs, l.gs.CmdArgs[1:])
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Binary provisioning is enabled but it failed to analyze the dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		l.gs.OSExit(1)
		return // this is required for testing
	}

	//  if the command does not have dependencies nor a custom build is required
	if !customBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		l.commandExecutor.run(l.gs)
		return
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("Binary Provisioning experimental feature is enabled." +
			" The current k6 binary doesn't satisfy all dependencies, it's required to" +
			" provision a custom binary.")

	customBinary, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to provision a k6 binary with required dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		l.gs.OSExit(1)
		return
	}

	customBinary.run(l.gs)
}

// customBinary runs the requested commands
// on a different binary on a subprocess passing the original arguments
type customBinary struct {
	// path represents the local file path
	// on the file system of the binary
	path string
}

//nolint:forbidigo
func (b *customBinary) run(gs *state.GlobalState) {
	cmd := exec.CommandContext(gs.Ctx, b.path, gs.CmdArgs[1:]...) //nolint:gosec

	// we pass os stdout, err, in because passing them from GlobalState changes how
	// the subprocess detects the type of terminal
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	// Copy environment variables to the k6 process and skip binary provisioning feature flag to disable it.
	// If not disabled, then the executed k6 binary would enter an infinite loop, where it continuously
	// process the input script, detect dependencies, and retrigger provisioning.
	env := []string{}
	for k, v := range gs.Env {
		if k == state.BinaryProvisioningFeatureFlag {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// handle signals
	sigC := make(chan os.Signal, 2)
	gs.SignalNotify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	gs.Logger.Debug("Launching the provisioned k6 binary")

	if err := cmd.Start(); err != nil {
		gs.Logger.
			WithError(err).
			Error("Failed to run the provisioned k6 binary")
		gs.OSExit(1)
	}

	// wait for the subprocess to end
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	for {
		select {
		case err := <-done:
			rc := 0
			if err != nil {
				rc = 1
				var eerr *exec.ExitError
				if errors.As(err, &eerr) {
					rc = eerr.ExitCode()
				}
			}
			gs.OSExit(rc)
		case sig := <-sigC:
			gs.Logger.
				WithField("signal", sig.String()).
				Debug("Signal received, waiting for the subprocess to handle it and return.")
		}
	}
}

// currentBinary runs the requested commands on the current binary
type currentBinary struct{}

func (b *currentBinary) run(gs *state.GlobalState) {
	ExecuteWithGlobalState(gs)
}

// customBuildRequired checks if the build is required
// it's required if there is one or more dependencies other than k6 itself
// or if the required k6 version is not satisfied by the current binary's version
// TODO: get the version of any built-in extension and check if they satisfy the dependencies
func customBuildRequired(baseK6Version string, deps k6deps.Dependencies) bool {
	if len(deps) == 0 {
		return false
	}

	// Early return if there are multiple dependencies
	if len(deps) > 1 {
		return true
	}

	k6Dependency, hasK6 := deps["k6"]

	// Early return if there's exactly one non-k6 dependency
	if !hasK6 {
		return true
	}

	// Ignore k6 dependency if nil
	if k6Dependency == nil || k6Dependency.Constraints == nil {
		return false
	}

	k6Ver, err := semver.NewVersion(baseK6Version)
	if err != nil {
		// ignore if baseK6Version is not a valid sem ver (e.g. a development version)
		return true
	}

	// if the current version satisfies the constrains, binary provisioning is not required
	return !k6Dependency.Constraints.Check(k6Ver)
}

// k6buildProvision returns the path to a k6 binary that satisfies the dependencies and the list of versions it provides
func k6buildProvision(gs *state.GlobalState, deps k6deps.Dependencies) (commandExecutor, error) {
	token, err := extractToken(gs)
	if err != nil {
		gs.Logger.WithError(err).Debug("Failed to get a valid token")
	}

	if token == "" {
		return nil, errors.New("k6 cloud token is required when the Binary provisioning feature is enabled." +
			" Set K6_CLOUD_TOKEN environment variable or execute the `k6 cloud login` command")
	}

	config := k6provider.Config{
		BuildServiceURL:  gs.Flags.BuildServiceURL,
		BuildServiceAuth: token,
		BinaryCacheDir:   gs.Flags.BinaryCache,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return nil, err
	}

	binary, err := provider.GetBinary(gs.Ctx, deps)
	if err != nil {
		return nil, err
	}

	gs.Logger.
		Info("A new k6 binary has been provisioned with version(s): ", formatDependencies(binary.Dependencies))

	return &customBinary{binary.Path}, nil
}

func formatDependencies(deps map[string]string) string {
	buffer := &bytes.Buffer{}
	for dep, version := range deps {
		fmt.Fprintf(buffer, "%s:%s ", dep, version)
	}
	return strings.Trim(buffer.String(), " ")
}

// extractToken gets the cloud token required to access the build service
// from the environment or from the config file
func extractToken(gs *state.GlobalState) (string, error) {
	diskConfig, err := readDiskConfig(gs)
	if err != nil {
		return "", err
	}

	config, _, err := cloudapi.GetConsolidatedConfig(diskConfig.Collectors["cloud"], gs.Env, "", nil, nil)
	if err != nil {
		return "", err
	}

	return config.Token.String, nil
}

// analyze returns the dependencies for the command to be executed.
// Presently, only the k6 input script or archive (if any) is passed to k6deps for scanning.
// TODO: if k6 receives the input from stdin, it is not used for scanning because we don't know
// if it is a script or an archive
func analyze(gs *state.GlobalState, args []string) (k6deps.Dependencies, error) {
	dopts := &k6deps.Options{
		LookupEnv: func(key string) (string, bool) { v, ok := gs.Env[key]; return v, ok },
		Manifest:  k6deps.Source{Ignore: true},
	}

	if !isAnalysisRequired(args) {
		gs.Logger.
			Debug("The command to execute does not require dependency analysis.")
		return k6deps.Dependencies{}, nil
	}

	scriptname := scriptNameFromArgs(args)
	if len(scriptname) == 0 {
		gs.Logger.
			Debug("The command did not receive an input script.")
		return nil, errScriptNotFound
	}

	if scriptname == "-" {
		gs.Logger.
			Debug("Test script provided by Stdin is not yet supported from Binary provisioning feature.")
		return nil, errUnsupportedFeature
	}

	if _, err := gs.FS.Stat(scriptname); err != nil {
		gs.Logger.
			WithField("path", scriptname).
			WithError(err).
			Debug("The requested test script's file is not available on the file system.")
		return nil, errScriptNotFound
	}

	if strings.HasSuffix(scriptname, ".tar") {
		dopts.Archive.Name = scriptname
	} else {
		dopts.Script.Name = scriptname
	}

	return k6deps.Analyze(dopts)
}

// isAnalysisRequired searches for the command and returns a boolean indicating if dependency analysis is required
func isAnalysisRequired(args []string) bool {
	// return early if no arguments passed
	if len(args) == 0 {
		return false
	}

	// search for a command that requires binary provisioning and then get the target script or archive
	// we handle cloud login subcommand as a special case because it does not require binary provisioning
	for i, arg := range args {
		switch arg {
		case "--help", "-h":
			return false
		case "cloud":
			for _, arg = range args[i+1:] {
				if arg == "login" || arg == "--help" || arg == "-h" {
					return false
				}
			}
			return true
		case "archive", "inspect":
			return true
		}
	}

	// not found
	return false
}

// scriptNameFromArgs returns the file name passed as input and true if it's a valid script name
func scriptNameFromArgs(args []string) string {
	// return early if no arguments passed
	if len(args) == 0 {
		return ""
	}

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if arg == "-" { // we are running a script from stdin
				return arg
			}
			continue
		}
		if strings.HasSuffix(arg, ".js") ||
			strings.HasSuffix(arg, ".tar") ||
			strings.HasSuffix(arg, ".ts") {
			return arg
		}
	}

	// not found
	return ""
}
