package k6deps

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/k6pack"
)

//nolint:forbidigo
func loadMetadata(dir string, opts *Options) error {
	var meta archiveMetadata

	data, err := os.ReadFile(filepath.Join(filepath.Clean(dir), "metadata.json"))
	if err != nil {
		return err
	}

	if err = json.Unmarshal(data, &meta); err != nil {
		return err
	}

	opts.Manifest.Ignore = true // no manifest (yet) in archive

	opts.Script.Name = filepath.Join(
		dir,
		"file",
		filepath.FromSlash(strings.TrimPrefix(meta.Filename, "file:///")),
	)

	if value, found := meta.Env[EnvDependencies]; found {
		opts.Env.Name = EnvDependencies
		opts.Env.Contents = []byte(value)
	} else {
		opts.Env.Ignore = true
	}

	contents, err := os.ReadFile(opts.Script.Name)
	if err != nil {
		return err
	}

	script, _, err := k6pack.Pack(string(contents), &k6pack.Options{Filename: opts.Script.Name})
	if err != nil {
		return err
	}

	opts.Script.Contents = script

	return nil
}

type archiveMetadata struct {
	Filename string            `json:"filename"`
	Env      map[string]string `json:"env"`
}

const maxFileSize = 1024 * 1024 * 10 // 10M

//nolint:forbidigo
func extractArchive(dir string, input io.Reader) error {
	reader := tar.NewReader(input)

	for {
		header, err := reader.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		target := filepath.Join(dir, filepath.Clean(filepath.FromSlash(header.Name)))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}

		case tar.TypeReg:
			if shouldSkip(target) {
				continue
			}

			file, err := os.OpenFile(filepath.Clean(target), os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)) //nolint:gosec
			if err != nil {
				return err
			}

			if _, err := io.CopyN(file, reader, maxFileSize); err != nil && !errors.Is(err, io.EOF) {
				return err
			}

			if err = file.Close(); err != nil {
				return err
			}

		// if it is a link or symlink, we copy the content of the linked file to the target
		// we assume the linked file was already processed and exists in the directory.
		case tar.TypeLink, tar.TypeSymlink:
			if shouldSkip(target) {
				continue
			}

			linkedFile := filepath.Join(dir, filepath.Clean(filepath.FromSlash(header.Linkname)))
			if err := followLink(linkedFile, target); err != nil {
				return err
			}
		}
	}
}

// indicates if the file should be skipped during extraction
// we skip csv files and .json except metadata.json
func shouldSkip(target string) bool {
	ext := filepath.Ext(target)
	return ext == ".csv" || (ext == ".json" && filepath.Base(target) != "metadata.json")
}

//nolint:forbidigo
func followLink(linkedFile string, target string) error {
	source, err := os.Open(filepath.Clean(linkedFile))
	if err != nil {
		return err
	}
	defer source.Close() //nolint:errcheck

	// we need to get the lined file info to create the target file with the same permissions
	info, err := source.Stat()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, info.Mode()) //nolint:gosec
	if err != nil {
		return err
	}

	_, err = io.Copy(file, source)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}
