package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/lib/fsext"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Options struct {
	// BuildServiceURL contains the URL of the k6 build service to be used.
	// If the value is not nil, the k6 binary is built using the build service instead of the local build.
	BuildServiceURL string
	// BuildServiceToken contains the token to be used to authenticate with the build service.
	// Defaults to K6_CLOUD_TOKEN environment variable is set, or the value stored in the k6 config file.
	BuildServiceToken string
}

type provider struct {
	opts    *Options
	gs      *state.GlobalState
	k6Exec  string
	cleanup func() error
	ppre    func(c *cobra.Command, args []string) error
}

// noopPPRE is a no-op PersistentPreRunE function.
func noopPPRE(c *cobra.Command, args []string) error { return nil }

func Install(gs *state.GlobalState, cmd *cobra.Command) {
	// for root this is never the case but leave in case we want wo install in a subcommand instead
	preRunE := cmd.PersistentPreRunE
	if preRunE == nil {
		preRunE = noopPPRE
	}

	provider := provider{
		gs:   gs,
		ppre: preRunE,
		opts: &Options{
			BuildServiceURL:   gs.Env["K6_BUILD_SERVICE_URL"],
			BuildServiceToken: gs.Env["K6_CLOUD_TOKEN"],
		},
	}

	// install the provider as the persistent pre-run function for all commands
	cmd.PersistentPreRunE = provider.persistentPreRunE
}

func (p *provider) persistentPreRunE(cmd *cobra.Command, args []string) error {
	if !slices.Contains([]string{"run", "archive", "inspect", "cloud"}, cmd.Name()) {
		return p.ppre(cmd, args)
	}

	deps, err := p.analyze(cmd, args)
	if err != nil {
		return err
	}

	if !isCustomBuildRequired(deps) {
		return p.ppre(cmd, args)
	}

	p.gs.Logger.Info("fetching k6 binary")

	k6exec, cleanup, err := p.provision(cmd.Context(), deps)
	if err != nil {
		return err
	}

	p.k6Exec = k6exec
	p.cleanup = cleanup

	cmd.RunE = p.runE

	return nil
}

func (p *provider) runE(cmd *cobra.Command, _ []string) error {
	k6Cmd := exec.CommandContext(cmd.Context(), p.k6Exec, p.gs.CmdArgs[1:]...) //nolint:gosec
	defer p.cleanup()

	k6Cmd.Stderr = p.gs.Stderr
	k6Cmd.Stdout = p.gs.Stdout
	k6Cmd.Stdin = p.gs.Stdin

	return k6Cmd.Run()
}

func (p *provider) analyze(cmd *cobra.Command, args []string) (k6deps.Dependencies, error) {
	depsOpts := p.newDepsOptions(cmd, args)

	// we call Analyze before logging because it will return the name of the manifest, in any
	deps, err := k6deps.Analyze(depsOpts)

	// FIXME: only log the sources that are not empty
	p.gs.Logger.WithFields(
		logrus.Fields{
			"script":   depsOpts.Script.Name,
			"archive":  depsOpts.Archive.Name,
			"manifest": depsOpts.Manifest.Name,
			"env":      depsOpts.Env.Name,
		},
	).Debug("analyzing dependencies")

	if err == nil && len(deps) > 0 {
		p.gs.Logger.Debugf("found dependencies: %s", deps)
	}

	return deps, err
}

func (p *provider) newDepsOptions(cmd *cobra.Command, args []string) *k6deps.Options {
	dopts := &k6deps.Options{}

	if len(args) == 0 {
		return dopts
	}

	// FIXME: we assume that the only argument is the script name
	scriptname := args[0]

	if _, err := os.Stat(scriptname); err != nil { //nolint:forbidigo
		return dopts
	}

	if strings.HasSuffix(scriptname, ".tar") {
		dopts.Archive.Name = scriptname
	} else {
		dopts.Script.Name = scriptname
	}

	return dopts
}

func (p *provider) provision(ctx context.Context, deps k6deps.Dependencies) (string, func() error, error) {
	config := k6provider.Config{
		BuildServiceURL:  p.opts.BuildServiceURL,
		BuildServiceAuth: p.opts.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", nil, err
	}

	p.gs.Logger.Debug("fetching binary", "build service URL: ", p.opts.BuildServiceURL)

	binary, err := provider.GetBinary(ctx, deps)
	if err != nil {
		return "", nil, err
	}

	// Cut the query string from the download URL to reduce noise in the logs
	downloadURL, _, _ := strings.Cut(binary.DownloadURL, "?")

	p.gs.Logger.WithFields(logrus.Fields{
		"Path":         binary.Path,
		"dependencies": binary.Dependencies,
		"checksum":     binary.Checksum,
		"cached":       binary.Cached,
		"download URL": downloadURL},
	).Debug("binary fetched")

	// TODO: once k6provider implements the cleanup of binary return the proper cleanup function (pablochacin)
	return binary.Path, func() error { return nil }, nil
}

// NewOptions creates a new Options object.
func NewOptions(gs *state.GlobalState) *Options {
	return &Options{
		BuildServiceURL:   gs.Env["K6_BUILD_SERVICE_URL"],
		BuildServiceToken: extractToken(gs),
	}
}

func extractToken(gs *state.GlobalState) string {
	token, ok := gs.Env["K6_CLOUD_TOKEN"]
	if ok {
		return token
	}

	// load from config file
	config, err := loadConfig(gs)
	if err != nil {
		return ""
	}

	return config.Collectors.Cloud.Token
}

// a simple struct to quickly load the config file
type k6configFile struct {
	Collectors struct {
		Cloud struct {
			Token string `json:"token"`
		} `json:"cloud"`
	} `json:"collectors"`
}

// loadConfig loads the k6 config file from the given path or the default location.
// if using the default location and the file does not exist, it returns an empty config.
func loadConfig(gs *state.GlobalState) (k6configFile, error) {
	var (
		config k6configFile
		err    error
	)

	// if not exists, return empty config
	_, err = fsext.Exists(gs.FS, gs.Flags.ConfigFilePath)
	if err != nil {
		return config, nil //nolint:nilerr
	}

	buffer, err := fsext.ReadFile(gs.FS, gs.Flags.ConfigFilePath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file %q: %w", gs.Flags.ConfigFilePath, err)
	}

	err = json.Unmarshal(buffer, &config)
	if err != nil {
		return config, fmt.Errorf("failed to parse config file %q: %w", gs.Flags.ConfigFilePath, err)
	}

	return config, nil
}

// isCustomBuildRequired checks if the build is required
// it's required if there is no k6 dependency in deps
// or if the resolved version is not the base version
// or if there are more than one (not k6) dependency
func isCustomBuildRequired(deps k6deps.Dependencies) bool {
	if len(deps) == 0 {
		return false
	}

	// Early return if there are multiple dependencies
	if len(deps) > 1 {
		return true
	}

	k6Dependency, hasK6 := deps["k6"]

	// Early return if there's exactly one non-k6 dependency
	if len(deps) == 1 && !hasK6 {
		return true
	}

	// Get k6 version constraint if it exists
	v := k6deps.ConstraintsAny
	if hasK6 && k6Dependency != nil && k6Dependency.Constraints != nil {
		v = k6Dependency.Constraints.String()
	}

	// No build required when default version is used
	if v == k6deps.ConstraintsAny {
		return false
	}

	// No build required when using the base version
	return v != build.Version
}
