package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
)

const (
	// cloudExtensionsCatalog defines the extensions catalog for cloud supported extensions
	cloudExtensionsCatalog = "cloud"
	// communityExtensionsCatalog defines the extensions catalog for community extensions
	communityExtensionsCatalog = "oss"
)

var (
	errScriptNotFound     = errors.New("script not found")
	errUnsupportedFeature = errors.New("not supported")
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

	if !isCustomBuildRequired(deps, build.Version, ext.GetAll()) {
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
	cmd.Stdin = os.Stdin

	// Copy environment variables to the k6 process and skip binary provisioning feature flag to disable it.
	// This avoids unnecessary re-processing of dependencies in the sub-process.
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

// isCustomBuildRequired checks if there is at least one dependency that are not satisfied by the binary
// considering the version of k6 and any built-in extension
func isCustomBuildRequired(deps k6deps.Dependencies, k6Version string, exts []*ext.Extension) bool {
	// return early if there are no dependencies
	if len(deps) == 0 {
		return false
	}

	// collect modules that this binary contain, including k6 itself
	builtIn := map[string]string{"k6": k6Version}
	for _, e := range exts {
		builtIn[e.Name] = e.Version
	}

	for _, dep := range deps {
		version, provided := builtIn[dep.Name]
		// if the binary does not contain a required module, we need a custom
		if !provided {
			return true
		}

		// If dependency's constrain is null, assume it is "*" and consider it satisfied.
		// See https://github.com/grafana/k6deps/issues/91
		if dep.Constraints == nil {
			continue
		}

		semver, err := semver.NewVersion(version)
		if err != nil {
			// ignore built in module if version is not a valid sem ver (e.g. a development version)
			// if user wants to use this built-in, must disable binary provisioning
			return true
		}

		// if the current version does not satisfies the constrains, binary provisioning is required
		if !dep.Constraints.Check(semver) {
			return true
		}
	}

	return false
}

// k6buildProvisioner provisions a k6 binary that satisfies the dependencies using the k6build service
type k6buildProvisioner struct {
	gs *state.GlobalState
}

func newK6BuildProvisioner(gs *state.GlobalState) provisioner {
	return &k6buildProvisioner{gs: gs}
}

func (p *k6buildProvisioner) provision(deps k6deps.Dependencies) (commandExecutor, error) {
	buildSrv, err := getBuildServiceURL(p.gs.Flags, p.gs.Logger)
	if err != nil {
		return nil, err
	}

	config := k6provider.Config{
		BuildServiceURL: buildSrv,
		BinaryCacheDir:  p.gs.Flags.BinaryCache,
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

// return the URL to the build service based on the configuration flags defined
func getBuildServiceURL(flags state.GlobalFlags, logger *logrus.Logger) (string, error) { //nolint:forbidigo
	buildSrv := flags.BuildServiceURL
	buildSrvURL, err := url.Parse(buildSrv)
	if err != nil {
		return "", fmt.Errorf("invalid URL to binary provisioning build service: %w", err)
	}

	catalog := cloudExtensionsCatalog
	if flags.EnableCommunityExtensions {
		catalog = communityExtensionsCatalog
	}

	logger.
		Debugf("using the %q extensions catalog", catalog)

	return buildSrvURL.JoinPath(catalog).String(), nil
}

func formatDependencies(deps map[string]string) string {
	buffer := &bytes.Buffer{}
	for dep, version := range deps {
		fmt.Fprintf(buffer, "%s:%s ", dep, version)
	}
	return strings.Trim(buffer.String(), " ")
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

	scriptname := args[0]
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

// isAnalysisRequired returns a boolean indicating if dependency analysis is required for the command
func isAnalysisRequired(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "run", "archive", "inspect", "upload", "cloud":
		return true
	}

	return false
}
