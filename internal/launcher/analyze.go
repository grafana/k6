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

func scriptArg(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}

	cmd := args[0] // FIXME: here is the issue (if we run k6 -v run script.js)
	if cmd != "run" && cmd != "archive" && cmd != "inspect" && cmd != "cloud" {
		return "", false
	}

	if len(args) == 1 {
		return "", false
	}

	last := args[len(args)-1]
	if last[0] == '-' {
		return "", false
	}

	return last, true
}
