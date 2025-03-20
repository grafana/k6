package launcher

import (
	"strings"

	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
)

func analyze(gs *state.GlobalState, args []string) (k6deps.Dependencies, error) {
	return k6deps.Analyze(newDepsOptions(gs, args))
}

func newDepsOptions(gs *state.GlobalState, args []string) *k6deps.Options {
	dopts := &k6deps.Options{
		LookupEnv: func(key string) (string, bool) { v, ok := gs.Env[key]; return v, ok },
		// TODO: figure out if we need to set FindManifest

	}

	scriptname, hasScript := scriptArg(args)
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

// scriptArg returns the script name and true if it's a valid script name
func scriptArg(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}

	// Find the command position (run, archive, inspect, cloud)
	cmdPos := -1
	for i, arg := range args {
		if arg == "run" || arg == "archive" || arg == "inspect" || arg == "cloud" {
			cmdPos = i
			break
		}
	}

	if cmdPos == -1 {
		return "", false
	}

	// Handle "cloud run" special case
	startPos := cmdPos + 1
	if cmdPos+1 < len(args) && args[cmdPos] == "cloud" && args[cmdPos+1] == "run" {
		startPos = cmdPos + 2
	}

	// Look for script files (non-flag arguments with .js, or .tar extension)
	for i := startPos; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			if strings.HasSuffix(args[i], ".js") ||
				strings.HasSuffix(args[i], ".tar") ||
				strings.HasSuffix(args[i], ".ts") {
				return args[i], true
			}
		}
	}

	return "", false
}
