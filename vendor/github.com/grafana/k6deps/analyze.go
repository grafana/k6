package k6deps

import "encoding/json"

// Analyze searches, loads and analyzes the specified sources,
// extracting the k6 extensions and their version constraints.
// Note: if archive is specified, the other three sources will not be taken into account,
// since the archive may contain them.
func Analyze(opts *Options) (Dependencies, error) {
	if err := loadSources(opts); err != nil {
		return nil, err
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
	if len(src.Contents) == 0 {
		return empty
	}

	return func() (Dependencies, error) {
		var manifest struct {
			Dependencies Dependencies `json:"dependencies,omitempty"`
		}

		if err := json.Unmarshal(src.Contents, &manifest); err != nil {
			return nil, err
		}

		return filterInvalid(manifest.Dependencies), nil
	}
}

func scriptAnalyzer(src Source) analyzer {
	if len(src.Contents) == 0 {
		return empty
	}

	return func() (Dependencies, error) {
		var deps Dependencies

		if err := (&deps).UnmarshalJS(src.Contents); err != nil {
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
