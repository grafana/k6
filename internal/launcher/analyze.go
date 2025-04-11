package launcher

import (
	"slices"
	"strings"

	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
)

// newDepsOptions returns the options for dependency resolution.
// Presently, only the k6 input script or archive (if any) is passed to k6deps for scanning.
// TODO: if k6 receives the input from stdin, it is not used for scanning because we don't know
// if it is a script or an archive
func newDepsOptions(gs *state.GlobalState, args []string) *k6deps.Options {
	dopts := &k6deps.Options{
		LookupEnv: func(key string) (string, bool) { v, ok := gs.Env[key]; return v, ok },
		// TODO: figure out if we need to set FindManifest
	}

	scriptname, hasScript := scriptNameFromArgs(args)
	if !hasScript {
		return dopts
	}

	if _, err := gs.FS.Stat(scriptname); err != nil {
		gs.Logger.
			WithField("scriptname", scriptname).
			WithError(err).
			Error("failed to stat script")

		return dopts
	}

	if strings.HasSuffix(scriptname, ".tar") {
		dopts.Archive.Name = scriptname
	} else {
		dopts.Script.Name = scriptname
	}

	return dopts
}

// scriptNameFromArgs returns the file name passed as input and true if it's a valid script name
func scriptNameFromArgs(args []string) (string, bool) {
	// return early if no arguments passed
	if len(args) == 0 {
		return "", false
	}

	// search for a command that requires binary provisioning and then get the target script or archive
	for i, arg := range args {
		if slices.Contains([]string{"run", "archive", "inspect", "cloud"}, arg) {
			// Look for script files (non-flag arguments with .js, or .tar extension) in the reminder args
			for _, arg = range args[i+1:] {
				if strings.HasPrefix(arg, "-") {
					// TODO: it may be we are using stdin as a source. Handle this case
					continue
				}
				if strings.HasSuffix(arg, ".js") ||
					strings.HasSuffix(arg, ".tar") ||
					strings.HasSuffix(arg, ".ts") {
					return arg, true
				}
			}
			break
		}
	}

	// not found
	return "", false
}
