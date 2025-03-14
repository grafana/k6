package provision

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"

	"github.com/spf13/cobra"
)

type Options struct {
	// Manifest contains the properties of the manifest file to be analyzed.
	// If the Ignore property is not set and no manifest file is specified,
	// the package.json file closest to the script is searched for.
	Manifest k6deps.Source
	// Env contains the properties of the environment variable to be analyzed.
	// If the Ignore property is not set and no variable is specified,
	// the value of the variable named K6_DEPENDENCIES is read.
	Env k6deps.Source
	// LookupEnv function is used to query the value of the environment variable
	// specified in the Env option Name if the Contents of the Env option is empty.
	// If empty, os.LookupEnv will be used.
	LookupEnv func(key string) (value string, ok bool)
	// FindManifest function is used to find manifest file for the given scriptfile
	// if the Contents of Manifest option is empty.
	// If the scriptfile parameter is empty, FindManifest starts searching
	// for the manifest file from the current directory
	// If missing, the closest manifest file will be used.
	FindManifest func(scriptfile string) (filename string, ok bool, err error)
	// AppName contains the name of the application. It is used to define the default value of CacheDir.
	// If empty, it defaults to os.Args[0].
	AppName string
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
	preRunE func(c *cobra.Command, args []string) error
}

func noopPreRunE(c *cobra.Command, args []string) error { return nil }

func Install(opts *Options, gs *state.GlobalState, cmd *cobra.Command) {

	preRunE := cmd.PreRunE
	if preRunE == nil {
		preRunE = noopPreRunE
	}

	provider := provider{
		gs:      gs,
		opts:    opts,
		preRunE: preRunE,
	}

	cmd.PreRunE = provider.PreRunE
}

func (p *provider) PreRunE(cmd *cobra.Command, args []string) error {
	deps, err := p.analyze(cmd, args)
	if err != nil {
		return err
	}

	if !isCustomBuildRequired(deps) {
		// execute original's command PreRunE
		return p.preRunE(cmd, args)
	}

	slog.Info("fetching k6 binary")

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

	k6Cmd.Stderr = os.Stderr //nolint:forbidigo
	k6Cmd.Stdout = os.Stdout //nolint:forbidigo
	k6Cmd.Stdin = os.Stdin   //nolint:forbidigo

	return k6Cmd.Run()
}

func (p *provider) analyze(cmd *cobra.Command, args []string) (k6deps.Dependencies, error) {
	depsOpts := p.newDepsOptions(cmd, args)

	// we call Analyze before logging because it will return the name of the manifest, in any
	deps, err := k6deps.Analyze(depsOpts)

	slog.Debug("analyzing sources", depsOptsAttrs(depsOpts)...)

	if err == nil && len(deps) > 0 {
		slog.Debug("found dependencies", "deps", deps.String())
	}

	return deps, err
}

func (p *provider) newDepsOptions(cmd *cobra.Command, args []string) *k6deps.Options {
	dopts := &k6deps.Options{
		Env:          p.opts.Env,
		Manifest:     p.opts.Manifest,
		LookupEnv:    p.opts.LookupEnv,
		FindManifest: p.opts.FindManifest,
	}

	scriptname, hasScript := scriptArg(cmd, args)
	if !hasScript {
		return dopts
	}

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

func scriptArg(cmd *cobra.Command, args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}

	if !slices.Contains([]string{"run", "archive", "inspect", "cloud"}, cmd.Name()) {
		return "", false
	}

	// FIXME: we assume that the only argument is the script name
	return args[0], true
}

func depsOptsAttrs(opts *k6deps.Options) []any {
	attrs := []any{}

	if opts.Manifest.Name != "" {
		attrs = append(attrs, "Manifest", opts.Manifest.Name)
	}

	if opts.Archive.Name != "" {
		attrs = append(attrs, "Archive", opts.Archive.Name)
	}

	// ignore script if archive is present
	if opts.Archive.Name == "" && opts.Script.Name != "" {
		attrs = append(attrs, "Script", opts.Script.Name)
	}

	if opts.Env.Name != "" {
		attrs = append(attrs, "Env", opts.Env.Name)
	}

	return attrs
}

func (p *provider) provision(ctx context.Context, deps k6deps.Dependencies) (string, func() error, error) {
	config := k6provider.Config{}

	if p.opts != nil {
		config.BuildServiceURL = p.opts.BuildServiceURL
		config.BuildServiceAuth = p.opts.BuildServiceToken
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", nil, err
	}

	slog.Debug("fetching binary", "build service URL: ", p.opts.BuildServiceURL)

	binary, err := provider.GetBinary(ctx, deps)
	if err != nil {
		return "", nil, err
	}

	// Cut the query string from the download URL to reduce noise in the logs
	downloadURL, _, _ := strings.Cut(binary.DownloadURL, "?")
	slog.Debug("binary fetched",
		"Path: ", binary.Path,
		"dependencies", deps.String(),
		"checksum", binary.Checksum,
		"cached", binary.Cached,
		"download URL", downloadURL,
	)

	// TODO: once k6provider implements the cleanup of binary return the proper cleanup function (pablochacin)
	return binary.Path, func() error { return nil }, nil
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
