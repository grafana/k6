package k6deps

import (
	"bytes"
	"io"
	"os"

	"github.com/spf13/afero"

	"github.com/grafana/k6deps/internal/pack"
	"github.com/grafana/k6deps/internal/rootfs"
)

const (
	// EnvDependencies holds the name of the environment variable that describes additional dependencies.
	EnvDependencies = "K6_DEPENDENCIES"
)

// Source describes a generic dependency source.
// Such a source can be the k6 script, the manifest file, or an environment variable (e.g. K6_DEPENDENCIES).
type Source struct {
	// Name contains the name of the source (file, environment variable, etc.).
	Name string
	// Reader provides streaming access to the source content as an alternative to Contents.
	Reader io.ReadCloser
	// Contents contains the content of the source (e.g. script)
	Contents []byte
	// Ignore disables automatic search and processing of that source.
	Ignore bool
}

// IsEmpty returns true if the source is empty.
func (s *Source) IsEmpty() bool {
	return len(s.Contents) == 0 && s.Reader == nil && len(s.Name) == 0
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
	// If not provided, os.LookupEnv will be used.
	LookupEnv func(key string) (value string, ok bool)
	// Fs is the file system to use for accessing files. If not provided, os file system is used
	Fs afero.Fs
	// Root directory for searching for files. Must an absolute path. If omitted, CWD is used
	RootDir string
}

func (opts *Options) lookupEnv(key string) (string, bool) {
	if opts.LookupEnv != nil {
		return opts.LookupEnv(key)
	}

	return os.LookupEnv(key) //nolint:forbidigo
}

// returns the FS to use with this options
func (opts *Options) fs() (rootfs.FS, error) {
	var err error

	dir := opts.RootDir
	if dir == "" {
		dir, err = os.Getwd() //nolint:forbidigo
		if err != nil {
			return nil, err
		}
	}

	if opts.Fs == nil {
		return rootfs.NewFromDir(dir)
	}

	return rootfs.NewFromFS(dir, opts.Fs), nil
}

// Analyze searches, loads and analyzes the specified sources,
// extracting the k6 extensions and their version constraints.
// Note: if archive is specified, the other three sources will not be taken into account,
// since the archive may contain them.
func Analyze(opts *Options) (Dependencies, error) {
	var err error

	if !opts.Archive.Ignore && !opts.Archive.IsEmpty() {
		archiveAnalyzer, err := opts.archiveAnalyzer()
		if err != nil {
			return nil, err
		}
		return archiveAnalyzer.analyze()
	}

	manifestAnalyzer, err := opts.manifestAnalyzer()
	if err != nil {
		return nil, err
	}

	scriptAnalyzeer, err := opts.scriptAnalyzer()
	if err != nil {
		return nil, err
	}

	return newMergeAnalyzer(scriptAnalyzeer, manifestAnalyzer, opts.envAnalyzer()).analyze()
}

// scriptAnalyzer loads a script Source and alls its dependencies into the Script's content
// from either a file of a reader
func (opts *Options) scriptAnalyzer() (analyzer, error) {
	source, err := opts.loadSource(&opts.Script)
	if err != nil {
		return nil, err
	}
	defer source.Close() //nolint:errcheck

	contents := &bytes.Buffer{}
	_, err = contents.ReadFrom(source)
	if err != nil {
		return nil, err
	}

	fs, err := opts.fs()
	if err != nil {
		return nil, err
	}
	script, _, err := pack.Pack(contents.String(), &pack.Options{FS: fs, Filename: opts.Script.Name})
	if err != nil {
		return nil, err
	}

	return newScriptAnalyzer(io.NopCloser(bytes.NewReader(script))), nil
}

func (opts *Options) manifestAnalyzer() (analyzer, error) {
	source, err := opts.loadSource(&opts.Manifest)
	if err != nil {
		return nil, err
	}

	return newManifestAnalyzer(source), nil
}

func (opts *Options) archiveAnalyzer() (analyzer, error) {
	source, err := opts.loadSource(&opts.Archive)
	if err != nil {
		return nil, err
	}

	return newArchiveAnalyzer(source), nil
}

func (opts *Options) loadSource(s *Source) (io.ReadCloser, error) {
	var err error
	if s.Ignore || s.IsEmpty() {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	if len(s.Contents) != 0 {
		return io.NopCloser(bytes.NewReader(s.Contents)), nil
	}

	fs, err := opts.fs()
	if err != nil {
		return nil, err
	}
	reader := s.Reader
	if reader == nil {
		reader, err = fs.Open(s.Name)
		if err != nil {
			return nil, err
		}
	}

	return reader, nil
}

func (opts *Options) envAnalyzer() analyzer {
	if opts.Env.Ignore {
		return newEmptyAnalyzer()
	}

	if len(opts.Env.Contents) > 0 {
		content := io.NopCloser(bytes.NewBuffer(opts.Env.Contents))
		return newTextAnalyzer(content)
	}

	key := opts.Env.Name
	if len(key) == 0 {
		key = EnvDependencies
	}

	value, _ := opts.lookupEnv(key)

	content := io.NopCloser(bytes.NewBuffer([]byte(value)))
	return newTextAnalyzer(content)
}
