package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6provider"

	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/ext"
)

// commandExecutor executes the requested k6 command line command.
// It abstract the execution path from the concrete binary.
type commandExecutor interface {
	run(ctx context.Context, gs *state.GlobalState) error
}

// provisioner defines the interface for provisioning a commandExecutor for a set of dependencies
type provisioner interface {
	provision(ctx context.Context, deps map[string]string) (commandExecutor, error)
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
func (b *customBinary) run(ctx context.Context, gs *state.GlobalState) error {
	cmd := exec.CommandContext(ctx, b.path, gs.CmdArgs[1:]...) //nolint:gosec

	// we pass os stdout, err, in because passing them from GlobalState changes how
	// the subprocess detects the type of terminal
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	// If stdin was used by the analyze function, the content has been preserved
	// in `gs.Stdin` and should be passed to the command
	cmd.Stdin = gs.Stdin

	cmd.Env = buildSubprocessEnv(gs.Env)

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

// buildSubprocessEnv prepares environment variables for a provisioned k6 subprocess.
func buildSubprocessEnv(src map[string]string) []string {
	env := []string{}
	for k, v := range src {
		if k == state.AutoExtensionResolution {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	// Disable auto extension resolution in the subprocess to avoid
	// unnecessary re-processing of dependencies.
	env = append(env, state.AutoExtensionResolution+"=false")
	env = append(env, state.ProvisionHostVersion+"="+runtimeK6Version())
	return env
}

// used just to signal we shouldn't print error again
var errAlreadyReported = fmt.Errorf("already reported error")

// isHexString reports whether s is non-empty and contains only lowercase hex digits.
func isHexString(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	}) < 0
}

// versionSHA extracts the commit SHA from an already-parsed semver Version.
// It handles two forms:
//
//	build metadata:  v0.0.0+sha   → v.Metadata() = "sha"
//	pseudo-version:  v0.0.0-YYYYMMDDHHMMSS-sha → last segment of v.Prerelease()
func versionSHA(v *semver.Version) (string, bool) {
	if m := v.Metadata(); isHexString(m) {
		return m, true
	}
	if pre := v.Prerelease(); pre != "" {
		parts := strings.Split(pre, "-")
		if len(parts) > 1 {
			if last := parts[len(parts)-1]; isHexString(last) {
				return last, true
			}
		}
	}
	return "", false
}

// extractVersionSHA parses version and returns any embedded commit SHA.
//
//	v0.0.0-20241015123456-abcdef123456  → "abcdef123456", true
//	v0.0.0+abc123                       → "abc123",       true
//	v1.2.3                              → "",             false
func extractVersionSHA(version string) (string, bool) {
	v, err := semver.NewVersion(version)
	if err != nil {
		return "", false
	}
	return versionSHA(v)
}

// constraintVersionStr returns the bare version token from a single constraint
// string by skipping any leading operator characters. Module versions always
// start with 'v', so the first 'v' marks the start of the version.
func constraintVersionStr(cs string) string {
	if i := strings.IndexByte(cs, 'v'); i >= 0 {
		return cs[i:]
	}
	return ""
}

// pseudoVersionSatisfies reports whether version satisfies c by comparing the
// commit SHAs embedded in both strings. It only applies when:
//   - version contains a recognisable SHA (pseudo-version or build-metadata form)
//   - c is a single exact-version constraint whose version also contains a SHA
//   - one SHA is a prefix of the other (handles 7-char vs 12-char short SHAs)
func pseudoVersionSatisfies(version string, c *semver.Constraints) bool {
	sv, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	vSHA, ok := versionSHA(sv)
	if !ok {
		return false
	}
	satisfied, definitive := checkConstraintBySHA(vSHA, c)
	return definitive && satisfied
}

// unsatisfiedDependencyError describes the first dependency that the current
// binary cannot satisfy. version is empty when the dependency is absent entirely.
type unsatisfiedDependencyError struct {
	name       string
	version    string // built-in version; empty if not present
	constraint string // required constraint
}

func (e unsatisfiedDependencyError) Error() string {
	if e.version == "" {
		return fmt.Sprintf("dependency %q is not present in the current binary", e.name)
	}
	return fmt.Sprintf("dependency %q: version %q does not satisfy constraint %q",
		e.name, e.version, e.constraint)
}

// checkBuiltinDependencies returns the first dependency from deps that is not
// satisfied by the built-in modules of the current binary (k6 itself plus any
// compiled-in extensions). Returns nil when all dependencies are satisfied.
func checkBuiltinDependencies(deps dependencies, k6Version string, exts []*ext.Extension) error {
	if len(deps) == 0 {
		return nil
	}

	// collect modules that this binary contains, including k6 itself
	builtIn := map[string]string{"k6": k6Version}
	for _, e := range exts {
		builtIn[e.Name] = e.Version
	}

	for name, constraint := range deps {
		version, provided := builtIn[name]
		if !provided {
			return unsatisfiedDependencyError{name: name}
		}
		if err := checkVersionConstraint(name, version, constraint); err != nil {
			return err
		}
	}

	return nil
}

// checkVersionConstraint checks whether version satisfies constraint for a named dependency.
func checkVersionConstraint(name, version string, constraint *semver.Constraints) error {
	// If dependency's constraint is null, assume it is "*" and consider it satisfied.
	// See https://github.com/grafana/k6deps/issues/91
	if constraint == nil {
		return nil
	}

	sv, err := semver.NewVersion(version)
	if err != nil {
		// version is not valid semver (e.g. a pseudo-version); try SHA comparison.
		if pseudoVersionSatisfies(version, constraint) {
			return nil
		}
		return unsatisfiedDependencyError{name: name, version: version, constraint: constraint.String()}
	}

	// Semver ignores build metadata (v0.0.0+sha) in comparisons per the spec,
	// so a plain constraint.Check would treat v0.0.0+abc and v0.0.0+xyz as
	// equal. When both sides embed a commit SHA, compare them directly.
	if vSHA, ok := versionSHA(sv); ok {
		if satisfied, definitive := checkConstraintBySHA(vSHA, constraint); definitive {
			if satisfied {
				return nil
			}
			return unsatisfiedDependencyError{name: name, version: version, constraint: constraint.String()}
		}
	}

	if !constraint.Check(sv) {
		// Pre-release pseudo-versions (v0.0.0-timestamp-sha) parse as semver but
		// may fail range checks even when the constraint names the exact same
		// commit. Try SHA comparison as a last resort.
		if pseudoVersionSatisfies(version, constraint) {
			return nil
		}
		return unsatisfiedDependencyError{name: name, version: version, constraint: constraint.String()}
	}
	return nil
}

// checkConstraintBySHA checks if vSHA matches the SHA embedded in constraint.
// Returns (satisfied, definitive): definitive=false means the constraint has no
// embedded SHA and normal semver comparison should be used instead.
// SHA matching is only applied to exact-version constraints (operator "=" or no
// operator); all other operators fall back to normal semver comparison.
func checkConstraintBySHA(vSHA string, constraint *semver.Constraints) (bool, bool) {
	cs := constraint.String()
	// Compound constraints cannot be matched by SHA alone; refuse definitively
	// so the caller does not fall back to semver, which mishandles pseudo-versions.
	if strings.ContainsAny(cs, " ,") {
		return false, true
	}
	versionStr := constraintVersionStr(cs)
	operator := cs[:len(cs)-len(versionStr)]
	if operator != "" && operator != "=" {
		return false, false
	}
	cv, err := semver.NewVersion(versionStr)
	if err != nil {
		return false, false
	}
	cSHA, ok := versionSHA(cv)
	if !ok {
		return false, false
	}
	return strings.HasPrefix(vSHA, cSHA) || strings.HasPrefix(cSHA, vSHA), true
}

// buildProvisioner provisions a k6 binary that satisfies the dependencies using the k6build service.
type buildProvisioner struct {
	gs         *state.GlobalState
	cachedOnly bool // if true, only looks up already-cached binaries
}

func newBuildProvisioner(gs *state.GlobalState) provisioner {
	return &buildProvisioner{gs: gs}
}

// newCacheProvisioner returns a provisioner that only looks up already-cached binaries.
// A cache miss surfaces as [fs.ErrNotExist]; the build service is never asked to build.
func newCacheProvisioner(gs *state.GlobalState) provisioner {
	return &buildProvisioner{gs: gs, cachedOnly: true}
}

func (p *buildProvisioner) provision(ctx context.Context, deps map[string]string) (commandExecutor, error) {
	config := getProviderConfig(p.gs)

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return nil, err
	}

	getBinary := provider.GetBinary
	if p.cachedOnly {
		getBinary = provider.GetCachedBinary
	}

	binary, err := getBinary(ctx, deps)
	if err != nil {
		return nil, err
	}
	if !p.cachedOnly {
		p.gs.Logger.Info("A new k6 binary has been provisioned with version(s): ",
			formatDependencies(binary.Dependencies))
	}

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

	config, _, err := cloudapi.GetConsolidatedConfig(diskConfig.Collectors["cloud"], gs.Env, "", nil)
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
		dep := "k6"
		constraint := directive
		if strings.HasPrefix(directive, "with k6/x/") {
			directive = strings.TrimSpace(strings.TrimPrefix(directive, "with "))
			dep, constraint, _ = strings.Cut(directive, " ")
		}
		var con *semver.Constraints
		var err error
		if len(constraint) > 0 {
			con, err = semver.NewConstraint(constraint)
			if err != nil {
				return fmt.Errorf("error while parsing use directives constraint %q for %q in %q: %w", constraint, dep, name, err)
			}
		}

		err = deps.update(dep, con)
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

func parseManifest(manifestString string) (dependencies, error) {
	if manifestString == "" {
		return nil, nil //nolint:nilnil
	}

	manifestMap := make(map[string]string)
	if err := json.Unmarshal([]byte(manifestString), &manifestMap); err != nil {
		return nil, fmt.Errorf("invalid dependencies manifest %w", err)
	}
	return dependenciesFromMap(manifestMap)
}

// completionTimeout bounds the cache lookup for shell completion requests.
// The build service is reachable but not guaranteed responsive; without a
// deadline a stall would hang the shell on every TAB.
const completionTimeout = 3 * time.Second

// completeExtension handles a shell completion request for an unregistered
// extension subcommand. If the matching provisioned binary is already cached
// locally, it delegates the completion request to that binary. Otherwise it
// returns nil (no completions) so the shell does not hang waiting on a build.
func completeExtension(gs *state.GlobalState, extName string, prov provisioner) error {
	deps, err := dependenciesFromSubcommand(gs, extName)
	if err != nil {
		gs.Logger.WithError(err).Debug("Failed to build completion deps")
		return nil
	}

	ctx, cancel := context.WithTimeout(gs.Ctx, completionTimeout)
	defer cancel()

	bin, err := prov.provision(ctx, constraintsMapToProvisionDependency(deps))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		gs.Logger.WithError(err).Debug("Failed to check provisioner cache for completion request")
		return nil
	}

	return bin.run(ctx, gs)
}
