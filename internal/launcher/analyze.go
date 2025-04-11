package launcher

import (
	"archive/tar"
	"bytes"
	"os"
	"strings"

	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
)

// analyze looks for the input argument to the k6 command and analyzes its dependencies.
// If the command does not require an input file, it returns no dependencies
// It the cases when the argument is a file or when it comes from stdin it must determine its format
// (tar or a js) and pass the options to k6deps accordingly
func analyze(gs *state.GlobalState, args []string) (k6deps.Dependencies, error) {
	dopts := &k6deps.Options{}

	var err error

	scriptFile := scriptNameFromArgs(args)
	switch scriptFile {
	case "":
		return k6deps.Dependencies{}, nil
	case "-":
		scriptFile, err = readStdin(gs)
		if err != nil {
			gs.Logger.
				WithError(err).
				Error("failed to read stdin into tmp file")
			return nil, err
		}
		defer func() {
			if errd := os.Remove(scriptFile); errd != nil {
				gs.Logger.
					WithError(errd).
					Debug("failed to remove tmp file")
			}
		}()
	default:
		if _, err := gs.FS.Stat(scriptFile); err != nil {
			gs.Logger.
				WithField("scriptname", scriptFile).
				WithError(err).
				Error("failed to stat script")

			return nil, err
		}
	}

	if strings.HasSuffix(scriptFile, ".tar") {
		dopts.Archive.Name = scriptFile
	} else {
		dopts.Script.Name = scriptFile
	}

	return k6deps.Analyze(dopts)
}

// scriptNameFromArgs returns the argument passed as input to the k6 command
func scriptNameFromArgs(args []string) string {
	// return early if no arguments passed
	if len(args) == 0 {
		return ""
	}

	// search for a command that requires binary provisioning and then get the target script or archive
	for i, arg := range args {
		switch arg {
		case "run", "archive", "inspect", "cloud":
			// Look for script files (non-flag arguments with .js, or .tar extension) in the reminder args
			for _, arg = range args[i+1:] {
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
			return ""
		}
	}

	// not found
	return ""
}

// read the stdin into a tmp file
// TODO: implement this logic without using os package
func readStdin(gs *state.GlobalState) (string, error) {
	buffer := bytes.NewBuffer(nil)
	_, err := buffer.ReadFrom(gs.Stdin)
	if err != nil {
		return "", err
	}

	fileName := "k6*" + detectInputType(buffer.Bytes())
	tmp, err := os.CreateTemp(os.TempDir(), fileName)
	if err != nil {
		return "", err
	}
	defer func() {
		if errc := tmp.Close(); errc != nil {
			gs.Logger.
				WithError(err).
				Error("closing temporary file used to process stdin")
		}
	}()

	_, err = tmp.Write(buffer.Bytes())
	if err != nil {
		return "", err
	}

	// make the content of stdin available if we fallback to running without binary provisioning
	gs.Stdin = buffer

	return tmp.Name(), nil
}

// figure out the type of input from a reader. If we can't read it as a tar, assume it is js
func detectInputType(content []byte) string {
	if _, err := tar.NewReader(bytes.NewBuffer(content)).Next(); err == nil {
		return ".tar"
	}
	return ".js"
}
