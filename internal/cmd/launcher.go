package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6provider"

	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/ext"
)

// commandExecutor executes the requested k6 command line command.
// It abstract the execution path from the concrete binary.
type commandExecutor interface {
	run(*state.GlobalState) error
}

// provisioner defines the interface for provisioning a commandExecutor for a set of dependencies
type provisioner interface {
	provision(map[string]string) (commandExecutor, error)
}

func constraintsMapToProvisionDependency(deps map[string]*semver.Constraints) k6provider.Dependencies {
	result := make(k6provider.Dependencies)
	for name, constraint := range deps {
		if constraint == nil {
			// If dependency's constraint is nil, assume it is "*" and consider it satisfied.
			// See https://github.com/grafana/k6deps/issues/91
			result[name] = "*"
			continue
		}
		result[name] = constraint.String()
	}

	return result
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

	// Copy environment variables to the k6 process skipping auto extension resolution feature flag.
	env := []string{}
	for k, v := range gs.Env {
		if k == state.AutoExtensionResolution {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	// If auto extension resolution is enabled then
	// this avoids unnecessary re-processing of dependencies in the sub-process.
	env = append(env, state.AutoExtensionResolution+"=false")
	// legacy envvar used in versions v1.0.x and v1.1.x
	env = append(env, "K6_BINARY_PROVISIONING=false")
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
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				return errext.WithExitCodeIfNone(errAlreadyReported, exitcodes.ExitCode(exitError.ExitCode())) //nolint:gosec
			}
			return err
		case sig := <-sigC:
			gs.Logger.
				WithField("signal", sig.String()).
				Debug("Signal received, waiting for the subprocess to handle it and return.")
		}
	}
}

// used just to signal we shouldn't print error again
var errAlreadyReported = fmt.Errorf("already reported error")

// isCustomBuildRequired checks if there is at least one dependency that are not satisfied by the binary
// considering the version of k6 and any built-in extension
func isCustomBuildRequired(deps dependencies, k6Version string, exts []*ext.Extension) bool {
	if len(deps) == 0 {
		return false
	}

	// collect modules that this binary contain, including k6 itself
	builtIn := map[string]string{"k6": k6Version}
	for _, e := range exts {
		builtIn[e.Name] = e.Version
	}

	for name, constraint := range deps {
		version, provided := builtIn[name]
		// if the binary does not contain a required module, we need a custom
		if !provided {
			return true
		}

		// If dependency's constraint is null, assume it is "*" and consider it satisfied.
		// See https://github.com/grafana/k6deps/issues/91
		if constraint == nil {
			continue
		}

		semver, err := semver.NewVersion(version)
		if err != nil {
			// ignore built in module if version is not a valid sem ver (e.g. a development version)
			// if user wants to use this built-in, must disable the automatic extension resolution
			return true
		}

		// if the current version does not satisfies the constrains, binary provisioning is required
		if !constraint.Check(semver) {
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

func (p *k6buildProvisioner) provision(deps map[string]string) (commandExecutor, error) {
	config := getProviderConfig(p.gs)

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

func getProviderConfig(gs *state.GlobalState) k6provider.Config {
	config := k6provider.Config{
		BuildServiceURL: gs.Flags.BuildServiceURL,
		BinaryCacheDir:  gs.Flags.BinaryCache,
	}

	token, err := extractToken(gs)
	if err != nil {
		gs.Logger.WithError(err).Debug("Failed to get cloud token")
	}

	if token != "" {
		config.BuildServiceAuth = token
	}

	return config
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

func processUseDirectives(name string, text []byte, deps dependencies) error {
	directives := findDirectives(text)

	for _, directive := range directives {
		// normalize spaces
		directive = strings.ReplaceAll(directive, "  ", " ")
		if !strings.HasPrefix(directive, "use k6") {
			continue
		}
		directive = strings.TrimSpace(strings.TrimPrefix(directive, "use k6"))
		if !strings.HasPrefix(directive, "with k6/x/") {
			err := deps.update("k6", directive)
			if err != nil {
				return fmt.Errorf("error while parsing use directives in %q: %w", name, err)
			}
			continue
		}
		directive = strings.TrimSpace(strings.TrimPrefix(directive, "with "))
		dep, constraint, _ := strings.Cut(directive, " ")
		err := deps.update(dep, constraint)
		if err != nil {
			return fmt.Errorf("error while parsing use directives in %q: %w", name, err)
		}
	}

	return nil
}

func findDirectives(text []byte) []string {
	// parse #! at beginning of file
	if bytes.HasPrefix(text, []byte("#!")) {
		_, text, _ = bytes.Cut(text, []byte("\n"))
	}

	var result []string

	for i := 0; i < len(text); {
		r, width := utf8.DecodeRune(text[i:])
		switch {
		case unicode.IsSpace(r) || r == rune(';'): // skip all spaces and ;
			i += width
		case r == '"' || r == '\'': // string literals
			idx := bytes.IndexRune(text[i+width:], r)
			if idx < 0 {
				return result
			}
			result = append(result, string(text[i+width:i+width+idx]))
			i += width + idx + 1
		case bytes.HasPrefix(text[i:], []byte("//")):
			idx := bytes.IndexRune(text[i+width:], '\n')
			if idx < 0 {
				return result
			}
			i += width + idx + 1
		case bytes.HasPrefix(text[i:], []byte("/*")):
			idx := bytes.Index(text[i+width:], []byte("*/"))
			if idx < 0 {
				return result
			}
			i += width + idx + 2
		default:
			return result
		}
	}
	return result
}

func mergeManifest(deps map[string]string, manifestString string) (map[string]string, error) {
	if manifestString == "" {
		return deps, nil
	}

	manifest := make(map[string]string)
	if err := json.Unmarshal([]byte(manifestString), &manifest); err != nil {
		return nil, fmt.Errorf("invalid dependencies manifest %w", err)
	}

	result := make(map[string]string)
	for dep, constraint := range deps {
		result[dep] = constraint

		// if deps has a non default constrain, keep ip
		if constraint != "" && constraint != "*" {
			continue
		}

		// check if there's an override in the manifest
		manifestConstrain := manifest[dep]
		if manifestConstrain != "" && manifestConstrain != "*" {
			result[dep] = manifestConstrain
		}
	}

	return result, nil
}
