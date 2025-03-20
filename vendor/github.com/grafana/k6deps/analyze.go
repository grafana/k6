package k6deps

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
)

// Analyze searches, loads and analyzes the specified sources,
// extracting the k6 extensions and their version constraints.
// Note: if archive is specified, the other three sources will not be taken into account,
// since the archive may contain them.
func Analyze(opts *Options) (Dependencies, error) {
	if !opts.Archive.Ignore && !opts.Archive.IsEmpty() {
		return archiveAnalizer(opts.Archive)()
	}

	// if the manifest is not provided, we try to find it
	// if not found, it will be empty and ignored by the analyzer
	if !opts.Manifest.Ignore && opts.Manifest.IsEmpty() {
		if err := opts.setManifest(); err != nil {
			return nil, err
		}
	}

	if !opts.Script.Ignore {
		if err := loadScript(opts); err != nil {
			return nil, err
		}
	}

	if !opts.Env.Ignore {
		loadEnv(opts)
	}

	return mergeAnalyzers(
		scriptAnalyzer(opts.Script),
		manifestAnalyzer(opts.Manifest),
		envAnalyzer(opts.Env),
	)()
}

type analyzer func() (Dependencies, error)

func empty() (Dependencies, error) {
	return make(Dependencies), nil
}

func filterInvalid(from Dependencies) Dependencies {
	deps := make(Dependencies)

	for name, dep := range from {
		if reName.MatchString(name) {
			deps[name] = dep
		}
	}

	return deps
}

func manifestAnalyzer(src Source) analyzer {
	if src.IsEmpty() {
		return empty
	}

	return func() (Dependencies, error) {
		reader, closer, err := src.contentReader()

		// we tolerate the manifest file not existing
		if errors.Is(err, os.ErrNotExist) { //nolint:forbidigo
			return make(Dependencies), nil
		}

		if err != nil {
			return nil, err
		}
		defer closer() //nolint:errcheck

		var manifest struct {
			Dependencies Dependencies `json:"dependencies,omitempty"`
		}

		err = json.NewDecoder(reader).Decode(&manifest)
		if err != nil {
			return nil, err
		}

		return filterInvalid(manifest.Dependencies), nil
	}
}

func scriptAnalyzer(src Source) analyzer {
	if src.IsEmpty() {
		return empty
	}

	return func() (Dependencies, error) {
		var deps Dependencies

		reader, closer, err := src.contentReader()
		if err != nil {
			return nil, err
		}
		defer closer() //nolint:errcheck

		buffer := new(bytes.Buffer)
		_, err = buffer.ReadFrom(reader)
		if err != nil {
			return nil, err
		}

		if err := (&deps).UnmarshalJS(buffer.Bytes()); err != nil {
			return nil, err
		}

		return deps, nil
	}
}

func envAnalyzer(src Source) analyzer {
	if len(src.Contents) == 0 {
		return empty
	}

	return func() (Dependencies, error) {
		var deps Dependencies

		if err := (&deps).UnmarshalText(src.Contents); err != nil {
			return nil, err
		}

		return filterInvalid(deps), nil
	}
}

func archiveAnalizer(src Source) analyzer {
	if src.IsEmpty() {
		return empty
	}

	return func() (Dependencies, error) {
		input := src.Reader
		if input == nil {
			tar, err := os.Open(src.Name) //nolint:forbidigo
			if err != nil {
				return nil, err
			}
			defer tar.Close() //nolint:errcheck

			input = tar
		}

		return processArchive(input)
	}
}

func mergeAnalyzers(sources ...analyzer) analyzer {
	return func() (Dependencies, error) {
		deps := make(Dependencies)

		for _, src := range sources {
			dep, err := src()
			if err != nil {
				return nil, err
			}

			err = deps.Merge(dep)
			if err != nil {
				return nil, err
			}
		}

		return deps, nil
	}
}
