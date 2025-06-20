package k6deps

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"slices"
)

type archiveMetadata struct {
	Filename string            `json:"filename"`
	Env      map[string]string `json:"env"`
}

const maxFileSize = 1024 * 1024 * 10 // 10M

func processArchive(input io.Reader) (Dependencies, error) {
	reader := tar.NewReader(input)

	deps := Dependencies{}
	for {
		header, err := reader.Next()

		switch {
		case errors.Is(err, io.EOF):
			return deps, nil
		case err != nil:
			return nil, err
		case header == nil:
			continue
		}

		if header.Typeflag != tar.TypeReg || !shouldProcess(header.Name) {
			continue
		}

		// if the file is metadata.json, we extract the dependencies from the env
		if header.Name == "metadata.json" {
			d, err := analizeMetadata(reader)
			if err != nil {
				return nil, err
			}

			err = deps.Merge(d)
			if err != nil {
				return nil, err
			}
			continue
		}

		// analize the file content as an script
		target := filepath.Clean(filepath.FromSlash(header.Name))
		src := Source{
			Name:   target,
			Reader: io.LimitReader(reader, maxFileSize),
		}

		d, err := scriptAnalyzer(src)()
		if err != nil {
			return nil, err
		}

		err = deps.Merge(d)
		if err != nil {
			return nil, err
		}
		continue
	}
}

// indicates if the file should be processed during extraction
func shouldProcess(target string) bool {
	ext := filepath.Ext(target)
	return slices.Contains([]string{".js", ".ts"}, ext) || slices.Contains([]string{"metadata.json", "data"}, target)
}

// analizeMetadata extracts the dependencies from the metadata.json file
func analizeMetadata(input io.Reader) (Dependencies, error) {
	metadata := archiveMetadata{}
	if err := json.NewDecoder(input).Decode(&metadata); err != nil {
		return nil, err
	}

	if value, found := metadata.Env[EnvDependencies]; found {
		src := Source{
			Name:     EnvDependencies,
			Contents: []byte(value),
		}

		return envAnalyzer(src)()
	}

	return Dependencies{}, nil
}
