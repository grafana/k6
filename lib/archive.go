/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package lib

import (
	"archive/tar"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// An Archive is a rollup of all resources and options needed to reproduce a test identically elsewhere.
type Archive struct {
	// The runner to use, eg. "js".
	Type string `json:"type"`

	// Options to use.
	Options Options `json:"options"`

	// Filename and contents of the main file being executed.
	Filename string `json:"filename"`
	Data     []byte `json:"-"`

	// Working directory for resolving relative paths.
	Pwd string `json:"pwd"`

	// Archived filesystem.
	Scripts map[string][]byte `json:"-"` // included scripts
	Files   map[string][]byte `json:"-"` // non-script resources
}

func ReadArchive(in io.Reader) (*Archive, error) {
	r := tar.NewReader(in)
	arc := &Archive{
		Scripts: make(map[string][]byte),
		Files:   make(map[string][]byte),
	}

	for {
		hdr, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}

		data, err := ioutil.ReadAll(r)
		if err != nil {
			return nil, err
		}

		switch hdr.Name {
		case "metadata.json":
			if err := json.Unmarshal(data, &arc); err != nil {
				return nil, err
			}
			continue
		case "data":
			arc.Data = data
		}

		idx := strings.IndexRune(hdr.Name, '/')
		if idx == -1 {
			continue
		}
		pfx := hdr.Name[:idx]
		name := hdr.Name[idx+1:]
		if name != "" && name[0] == '_' {
			name = name[1:]
		}

		var dst map[string][]byte
		switch pfx {
		case "files":
			dst = arc.Files
		case "scripts":
			dst = arc.Scripts
		default:
			continue
		}

		dst[name] = data
	}

	return arc, nil
}

func (arc *Archive) Write(out io.Writer) error {
	w := tar.NewWriter(out)
	t := time.Now()

	metadata, err := json.MarshalIndent(arc, "", "  ")
	if err != nil {
		return err
	}
	w.WriteHeader(&tar.Header{
		Name:     "metadata.json",
		Mode:     0644,
		Size:     int64(len(metadata)),
		ModTime:  t,
		Typeflag: tar.TypeReg,
	})
	if _, err := w.Write(metadata); err != nil {
		return err
	}

	w.WriteHeader(&tar.Header{
		Name:     "data",
		Mode:     0644,
		Size:     int64(len(arc.Data)),
		ModTime:  t,
		Typeflag: tar.TypeReg,
	})
	if _, err := w.Write(arc.Data); err != nil {
		return err
	}

	arcfs := []struct {
		name  string
		files map[string][]byte
	}{
		{"scripts", arc.Scripts},
		{"files", arc.Files},
	}
	for _, entry := range arcfs {
		w.WriteHeader(&tar.Header{
			Name:     entry.name,
			Mode:     0755,
			ModTime:  t,
			Typeflag: tar.TypeDir,
		})

		// A couple of things going on here:
		// - You can't just create file entries, you need to create directory entries too.
		//   Figure out which directories are in use here.
		// - We want archives to be comparable by hash, which means the entries need to be written
		//   in the same order every time. Go maps are shuffled, so we need to sort lists of keys.
		foundDirs := make(map[string]bool)
		paths := make([]string, 0, len(entry.files))
		for path := range entry.files {
			paths = append(paths, path)
			dir := filepath.Dir(path)
			for {
				foundDirs[dir] = true
				idx := strings.LastIndexByte(dir, os.PathSeparator)
				if idx == -1 {
					break
				}
				dir = dir[:idx]
			}
		}
		dirs := make([]string, 0, len(foundDirs))
		for dirpath := range foundDirs {
			dirs = append(dirs, dirpath)
		}
		sort.Strings(paths)
		sort.Strings(dirs)

		for _, dirpath := range dirs {
			if dirpath == "" || dirpath[0] == '/' {
				dirpath = "_" + dirpath
			}
			w.WriteHeader(&tar.Header{
				Name:     filepath.Clean(entry.name + "/" + dirpath),
				Mode:     0755,
				ModTime:  t,
				Typeflag: tar.TypeDir,
			})
		}

		for _, path := range paths {
			data := entry.files[path]
			if path[0] == '/' {
				path = "_" + path
			}
			w.WriteHeader(&tar.Header{
				Name:     filepath.Clean(entry.name + "/" + path),
				Mode:     0644,
				Size:     int64(len(data)),
				ModTime:  t,
				Typeflag: tar.TypeReg,
			})
			if _, err := w.Write(data); err != nil {
				return err
			}
		}
	}

	return w.Close()
}
