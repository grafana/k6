package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/pflag"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/js"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
)

const optionsFileFlagName = "options-file"

// optionsFileValue is the value of --options-file. It rejects a second occurrence, because
// pflag's string flags silently keep the last one.
type optionsFileValue struct {
	path string
}

func (v *optionsFileValue) String() string { return v.path }

func (v *optionsFileValue) Type() string { return "file" }

func (v *optionsFileValue) Set(path string) error {
	if v.path != "" {
		return errors.New("can be passed only once")
	}
	if path == "" {
		return errors.New("requires a path")
	}
	v.path = path
	return nil
}

// optionsFileFlagSet returns the flag that switches on the options file mode, in which the
// test options come only from the file and the script's options export is ignored.
func optionsFileFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	flags.Var(&optionsFileValue{}, optionsFileFlagName,
		"read the test options from a JSON `file` and ignore the options exported by the script")
	return flags
}

// optionsFilePath returns the --options-file path and whether the mode is on.
// Commands without the flag, such as deps and cloud run, share the loading code and are
// never in the mode.
func optionsFilePath(flags *pflag.FlagSet) (string, bool) {
	f := flags.Lookup(optionsFileFlagName)
	if f == nil || !f.Changed {
		return "", false
	}
	return f.Value.String(), true
}

// readOptionsFile reads the test options from the options file. The file has the shape of
// the script's options export. Unlike there, unknown fields are an error.
func readOptionsFile(gs *state.GlobalState, path string) (lib.Options, error) {
	if !filepath.IsAbs(path) {
		pwd, err := gs.Getwd()
		if err != nil {
			return lib.Options{}, err
		}
		path = filepath.Join(pwd, path)
	}

	// Read through gs.FS, so the file does not end up in archives.
	data, err := fsext.ReadFile(gs.FS, path)
	if err != nil {
		return lib.Options{}, errext.WithExitCodeIfNone(
			fmt.Errorf("couldn't read the options file %q: %w", path, err), exitcodes.InvalidConfig)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return lib.Options{}, errext.WithExitCodeIfNone(
			fmt.Errorf("the options file %q is empty", path), exitcodes.InvalidConfig)
	}

	var opts lib.Options
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opts); err != nil {
		err = js.BeautifyOptionsJSONUnmarshalError(data, err, optionsFileLayerName)
		return lib.Options{}, errext.WithExitCodeIfNone(
			fmt.Errorf("couldn't parse the options file %q: %w", path, err), exitcodes.InvalidConfig)
	}

	return opts, nil
}
