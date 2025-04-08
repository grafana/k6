package k6deps

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/grafana/k6deps/internal/pack"
)

// EnvDependencies holds the name of the environment variable thet describes additional dependencies.
const EnvDependencies = "K6_DEPENDENCIES"

// Source describes a generic dependency source.
// Such a source can be the k6 script, the manifest file, or an environment variable (e.g. K6_DEPENDENCIES).
type Source struct {
	// Name contains the name of the source (file, environment variable, etc.).
	Name string
	// Reader provides streaming access to the source content as an alternative to Contents.
	Reader io.Reader
	// Contents contains the content of the source (e.g. script)
	Contents []byte
	// Ignore disables automatic search and processing of that source.
	Ignore bool
}

// IsEmpty returns true if the source is empty.
func (s *Source) IsEmpty() bool {
	return len(s.Contents) == 0 && s.Reader == nil && len(s.Name) == 0
}

func nopCloser() error {
	return nil
}

// contentReader returns a reader for the source content.
func (s *Source) contentReader() (io.Reader, func() error, error) {
	if s.Reader != nil {
		return s.Reader, nopCloser, nil
	}

	if len(s.Contents) > 0 {
		return bytes.NewReader(s.Contents), nopCloser, nil
	}

	fileName, err := filepath.Abs(s.Name)
	if err != nil {
		return nil, nil, err
	}

	file, err := os.Open(filepath.Clean(fileName)) //nolint:forbidigo
	if err != nil {
		return nil, nil, err
	}

	return file, file.Close, nil
}

// Options contains the parameters of the dependency analysis.
type Options struct {
	// Script contains the properties of the k6 test script to be analyzed.
	// If the name is specified, but no content is provided, the script is read from the file.
	// Any script file referenced will be recursively loaded and the dependencies merged.
	// If the Ignore property is set, the script will not be analyzed.
	Script Source
	// Archive contains the properties of the k6 archive to be analyzed.
	// If archive is specified, the other three sources will not be taken into account,
	// since the archive may contain them.
	// It is assumed that the script and all dependencies are in the archive. No external dependencies are analyzed.
	// An archive is a tar file, which can be created using the k6 archive command.
	Archive Source
	// Manifest contains the properties of the manifest file to be analyzed.
	// If the Ignore property is not set and no manifest file is specified,
	// the package.json file closest to the script is searched for.
	Manifest Source
	// Env contains the properties of the environment variable to be analyzed.
	// If the Ignore property is not set and no variable is specified,
	// the value of the variable named K6_DEPENDENCIES is read.
	Env Source
	// LookupEnv function is used to query the value of the environment variable
	// specified in the Env option Name if the Contents of the Env option is empty.
	// If empty, os.LookupEnv will be used.
	LookupEnv func(key string) (value string, ok bool)
	// FindManifest function is used to find manifest file for the given script file
	// if the Contents of Manifest option is empty.
	// If the scriptfile parameter is empty, FindManifest starts searching
	// for the manifest file from the current directory
	// If missing, the closest manifest file will be used.
	FindManifest func(scriptfile string) (filename string, ok bool, err error)
}

func (opts *Options) lookupEnv(key string) (string, bool) {
	if opts.LookupEnv != nil {
		return opts.LookupEnv(key)
	}

	return os.LookupEnv(key) //nolint:forbidigo
}

func loadScript(opts *Options) error {
	if len(opts.Script.Name) == 0 || len(opts.Script.Contents) > 0 || opts.Script.Ignore {
		return nil
	}

	scriptfile, err := filepath.Abs(opts.Script.Name)
	if err != nil {
		return err
	}

	contents, err := os.ReadFile(scriptfile) //nolint:forbidigo,gosec
	if err != nil {
		return err
	}

	script, _, err := pack.Pack(string(contents), &pack.Options{Filename: scriptfile})
	if err != nil {
		return err
	}

	opts.Script.Name = scriptfile
	opts.Script.Contents = script

	return nil
}

func loadEnv(opts *Options) {
	if len(opts.Env.Contents) > 0 || opts.Env.Ignore {
		return
	}

	key := opts.Env.Name
	if len(key) == 0 {
		key = EnvDependencies
	}

	value, found := opts.lookupEnv(key)
	if !found || len(value) == 0 {
		return
	}

	opts.Env.Name = key
	opts.Env.Contents = []byte(value)
}

func (opts *Options) setManifest() error {
	// if the manifest is not provided, we try to find it
	// starting from the location of the script
	path, found, err := findManifest(opts.Script.Name)
	if err != nil {
		return err
	}
	if found {
		opts.Manifest.Name = path
	}

	return nil
}

func findManifest(filename string) (string, bool, error) {
	if len(filename) == 0 {
		filename = "any_file"
	}

	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", false, err
	}

	for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
		filename := filepath.Clean(filepath.Join(dir, "package.json"))
		if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) { //nolint:forbidigo
			return filename, err == nil, err
		}

		if dir[len(dir)-1] == filepath.Separator {
			break
		}
	}

	return "", false, nil
}
