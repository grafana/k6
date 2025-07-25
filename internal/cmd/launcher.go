package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
)

// commandExecutor executes the requested k6 command line command.
// It abstract the execution path from the concrete binary.
type commandExecutor interface {
	run(*state.GlobalState) error
}

// provisioner defines the interface for provisioning a commandExecutor for a set of dependencies
type provisioner interface {
	provision(k6deps.Dependencies) (commandExecutor, error)
}

// launcher is a k6 launcher. It analyses the requirements of a k6 execution,
// then if required, it provisions a binary executor to satisfy the requirements.
type launcher struct {
	gs              *state.GlobalState
	provisioner     provisioner
	commandExecutor commandExecutor
}

// newLauncher creates a new Launcher from a GlobalState using the default provision function
func newLauncher(gs *state.GlobalState) *launcher {
	return &launcher{
		gs:          gs,
		provisioner: newK6BuildProvisioner(gs),
	}
}

// launch analyzies the command to be executed and its input (e.g. script) to identify its dependencies.
// If it has dependencies that cannot be satisfied by the current binary, it obtains a custom commandExecutor
// usign the provision function and delegates the execution of the command to this  commandExecutor.
// On the contrary, continues with the execution of the command in the current binary.
func (l *launcher) launch(cmd *cobra.Command, args []string) error {
	if !isAnalysisRequired(cmd) {
		l.gs.Logger.
			WithField("command", cmd.Name()).
			Debug("command does not require dependency analysis")
		return nil
	}

	deps, err := analyze(l.gs, args)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Binary provisioning is enabled but it failed to analyze the dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		return err
	}

	//  if the command does not have dependencies nor a custom build is required
	if !isCustomBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		return nil
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("Binary Provisioning experimental feature is enabled." +
			" The current k6 binary doesn't satisfy all dependencies, it's required to" +
			" provision a custom binary.")

	customBinary, err := l.provisioner.provision(deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to provision a k6 binary with required dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		return err
	}

	l.commandExecutor = customBinary

	// override command's RunE method to be processed by the command executor
	cmd.RunE = l.runE

	return nil
}

// runE executes the k6 command using a command executor
func (l *launcher) runE(_ *cobra.Command, _ []string) error {
	return l.commandExecutor.run(l.gs)
}

// customBinary runs the requested commands on a different binary on a subprocess passing the
// original arguments
type customBinary struct {
	// path represents the local file path
	// on the file system of the binary
	path string
}

//nolint:forbidigo
func (b *customBinary) run(gs *state.GlobalState) error {
	cmd := exec.CommandContext(gs.Ctx, b.path, gs.CmdArgs[1:]...) //nolint:gosec

	// we pass os stdout, err, in because passing them from GlobalState changes how
	// the subprocess detects the type of terminal
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	// If stdin was used by the analyze function, the content has been preserved
	// in `gs.Stdin` and should be passed to the command
	cmd.Stdin = gs.Stdin

	// Copy environment variables to the k6 process and skip binary provisioning feature flag to disable it.
	// If not disabled, then the executed k6 binary would enter an infinite loop, where it continuously
	// process the input script, detect dependencies, and retrigger provisioning.
	// This can be avoided by checking if the current binary has already extensions that
	// satisfies the dependencies. See comment in isCustomBuildRequired function.
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
		return err
	}

	// wait for the subprocess to end
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	for {
		select {
		case err := <-done:
			return err
		case sig := <-sigC:
			gs.Logger.
				WithField("signal", sig.String()).
				Debug("Signal received, waiting for the subprocess to handle it and return.")
		}
	}
}

// isCustomBuildRequired checks if the build is required
// it's required if there is one or more dependencies other than k6 itself
// or if the required k6 version is not satisfied by the current binary's version
// TODO: get the version of any built-in extension and check if they satisfy the dependencies
// (see https://github.com/grafana/k6/issues/4697)
func isCustomBuildRequired(baseK6Version string, deps k6deps.Dependencies) bool {
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

// k6buildProvisioner provisions a k6 binary that satisfies the dependencies using the k6build service
type k6buildProvisioner struct {
	gs *state.GlobalState
}

func newK6BuildProvisioner(gs *state.GlobalState) provisioner {
	return &k6buildProvisioner{gs: gs}
}

func (p *k6buildProvisioner) provision(deps k6deps.Dependencies) (commandExecutor, error) {
	token, err := extractToken(p.gs)
	if err != nil {
		p.gs.Logger.WithError(err).Debug("Failed to get a valid token")
	}

	if token == "" {
		return nil, errors.New("k6 cloud token is required when the Binary provisioning feature is enabled." +
			" Set K6_CLOUD_TOKEN environment variable or execute the `k6 cloud login` command")
	}

	config := k6provider.Config{
		BuildServiceURL:  p.gs.Flags.BuildServiceURL,
		BuildServiceAuth: token,
		BinaryCacheDir:   p.gs.Flags.BinaryCache,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return nil, err
	}

	binary, err := provider.GetBinary(p.gs.Ctx, deps)
	if err != nil {
		return nil, err
	}

	p.gs.Logger.
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

	sourceRootPath := args[0]
	gs.Logger.WithField("source", "sourceRootPath").
		Debug("Launcher is resolving and reading the test's script")
	src, _, pwd, err := readSource(gs, sourceRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading source for analysis %w", err)
	}

	// if sourceRooPath is stdin ('-') we need to preserve the content
	if sourceRootPath == "-" {
		gs.Stdin = bytes.NewBuffer(src.Data)
	}

	if strings.HasSuffix(sourceRootPath, ".tar") {
		dopts.Archive.Contents = src.Data
	} else {
		if !filepath.IsAbs(sourceRootPath) {
			sourceRootPath = filepath.Join(pwd, sourceRootPath)
		}
		dopts.Script.Name = sourceRootPath
		dopts.Script.Contents = src.Data
		dopts.Fs = gs.FS
	}

	return k6deps.Analyze(dopts)
}

// isAnalysisRequired returns a boolean indicating if dependency analysis is required for the command
func isAnalysisRequired(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "run":
		// exclude `k6 cloud run` command
		if cmd.Parent() != nil && cmd.Parent().Name() == "cloud" {
			return true
		}
		return false
	case "archive", "inspect", "upload", "cloud":
		return true
	}

	// not found
	return false
}
