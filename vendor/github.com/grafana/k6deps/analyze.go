package k6deps

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// analyzer defines the interface for a dependency analyzer
type analyzer interface {
	analyze() (Dependencies, error)
}

// emptyAnalyzer returns empty Dependencies
type emptyAnalyzer struct{}

func newEmptyAnalyzer() analyzer {
	return &emptyAnalyzer{}
}

func (e *emptyAnalyzer) analyze() (Dependencies, error) {
	return make(Dependencies), nil
}

type manifestAnalyzer struct {
	src io.ReadCloser
}

func newManifestAnalyzer(src io.ReadCloser) analyzer {
	return &manifestAnalyzer{src: src}
}

func (m *manifestAnalyzer) analyze() (Dependencies, error) {
	defer m.src.Close() //nolint:errcheck

	var manifest struct {
		Dependencies Dependencies `json:"dependencies,omitempty"`
	}

	err := json.NewDecoder(m.src).Decode(&manifest)

	if err == nil {
		return filterInvalid(manifest.Dependencies), nil
	}

	if errors.Is(err, io.EOF) {
		return make(Dependencies), nil
	}

	return nil, err
}

type scriptAnalyzer struct {
	src io.ReadCloser
}

func newScriptAnalyzer(src io.ReadCloser) analyzer {
	return &scriptAnalyzer{src: src}
}

func (s *scriptAnalyzer) analyze() (Dependencies, error) {
	var deps Dependencies
	defer s.src.Close() //nolint:errcheck

	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(s.src); err != nil {
		return nil, err
	}

	if err := (&deps).UnmarshalJS(buffer.Bytes()); err != nil {
		return nil, err
	}

	return deps, nil
}

// textAnalyzer analyzes dependencies from a text format
type textAnalyzer struct {
	src io.ReadCloser
}

func newTextAnalyzer(src io.ReadCloser) analyzer {
	return &textAnalyzer{src: src}
}

func (e *textAnalyzer) analyze() (Dependencies, error) {
	var deps Dependencies
	content := &bytes.Buffer{}
	_, err := content.ReadFrom(e.src)
	if err != nil {
		return nil, err
	}

	if content.Len() == 0 {
		return make(Dependencies), nil
	}

	if err := (&deps).UnmarshalText(content.Bytes()); err != nil {
		return nil, err
	}

	return filterInvalid(deps), nil
}

// archiveAnalizer analizes a k6 archive in .tar format
type archiveAnalizer struct {
	src io.ReadCloser
}

func newArchiveAnalyzer(src io.ReadCloser) analyzer {
	return &archiveAnalizer{src: src}
}

func (a *archiveAnalizer) analyze() (Dependencies, error) {
	defer a.src.Close() //nolint:errcheck

	return processArchive(a.src)
}

type mergeAnalyzer struct {
	analyzers []analyzer
}

func newMergeAnalyzer(analyzers ...analyzer) analyzer {
	return &mergeAnalyzer{analyzers: analyzers}
}

func (m *mergeAnalyzer) analyze() (Dependencies, error) {
	deps := make(Dependencies)

	for _, a := range m.analyzers {
		dep, err := a.analyze()
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

func filterInvalid(from Dependencies) Dependencies {
	deps := make(Dependencies)

	for name, dep := range from {
		if reName.MatchString(name) {
			deps[name] = dep
		}
	}

	return deps
}
