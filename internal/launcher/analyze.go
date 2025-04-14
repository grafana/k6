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

	// command has no script
	if len(args) == 0 {
		return dopts
	}

	// assume first argument is the script name
	scriptname := args[0]

	if scriptname == "-" {
		gs.Logger.
			Warn("binary provisioning is not supported when using input from stdin")
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
