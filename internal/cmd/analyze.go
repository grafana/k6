package cmd

import (
	"errors"
	"strings"

	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
)

var (
	errScriptNotFound     = errors.New("script not found")
	errUnsupportedFeature = errors.New("not supported")
)

// analyze returns the dependencies for the command to be executed.
// Presently, only the k6 input script or archive (if any) is passed to k6deps for scanning.
// TODO: if k6 receives the input from stdin, it is not used for scanning because we don't know
// if it is a script or an archive
func analyze(gs *state.GlobalState, args []string) (k6deps.Dependencies, error) {
	dopts := &k6deps.Options{
		LookupEnv: func(key string) (string, bool) { v, ok := gs.Env[key]; return v, ok },
	}

	if !isScriptRequired(args) {
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

// isScriptRequired searches for the command and returns a boolean indicating if it is required to pass a script or not
func isScriptRequired(args []string) bool {
	// return early if no arguments passed
	if len(args) == 0 {
		return false
	}

	// search for a command that requires binary provisioning and then get the target script or archive
	// we handle cloud login subcommand as a special case because it does not require binary provisioning
	for i, arg := range args {
		switch arg {
		case "cloud":
			for _, arg = range args[i+1:] {
				if arg == "login" {
					return false
				}
			}
			return true
		case "run", "archive", "inspect":
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
