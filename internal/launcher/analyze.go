package launcher

import (
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

	if scriptname == "-" {
		gs.Logger.
			Warn("Test script provided by Stdin is not yet supported from Binary provisioning feature.")
		return dopts
	}

	if _, err := gs.FS.Stat(scriptname); err != nil {
		gs.Logger.
			WithField("path", scriptname).
			WithError(err).
			Error("The requested test script's file is not available on the file system. Make sure that the correct name has been passed or that the file still exists on the file system.")

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
		switch arg {
		case "run", "archive", "inspect", "cloud":
			// Look for script files (non-flag arguments with .js, or .tar extension) in the reminder args
			for _, arg = range args[i+1:] {
				if strings.HasPrefix(arg, "-") {
					if arg == "-" { // we are running a script from stdin
						return arg, true
					}
					continue
				}
				if strings.HasSuffix(arg, ".js") ||
					strings.HasSuffix(arg, ".tar") ||
					strings.HasSuffix(arg, ".ts") {
					return arg, true
				}
			}
			return "", false
		}
	}

	// not found
	return "", false
}
