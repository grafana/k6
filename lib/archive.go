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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/loadimpact/k6/lib/fsext"
	"github.com/loadimpact/k6/loader"
	"github.com/spf13/afero"
)

//nolint: gochecknoglobals, lll
var (
	volumeRE  = regexp.MustCompile(`^[/\\]?([a-zA-Z]):(.*)`)
	sharedRE  = regexp.MustCompile(`^\\\\([^\\]+)`) // matches a shared folder in Windows before backslack replacement. i.e \\VMBOXSVR\k6\script.js
	homeDirRE = regexp.MustCompile(`(?i)^(/[a-zA-Z])?/(Users|home|Documents and Settings)/(?:[^/]+)`)
)

// NormalizeAndAnonymizePath Normalizes (to use a / path separator) and anonymizes a file path, by scrubbing usernames from home directories.
func NormalizeAndAnonymizePath(path string) string {
	path = filepath.Clean(path)

	p := volumeRE.ReplaceAllString(path, `/$1$2`)
	p = sharedRE.ReplaceAllString(p, `/nobody`)
	p = strings.Replace(p, "\\", "/", -1)
	return homeDirRE.ReplaceAllString(p, `$1/$2/nobody`)
}

func newNormalizedFs(fs afero.Fs) afero.Fs {
	return fsext.NewChangePathFs(fs, fsext.ChangePathFunc(func(name string) (string, error) {
		return NormalizeAndAnonymizePath(name), nil
	}))
}

// An Archive is a rollup of all resources and options needed to reproduce a test identically elsewhere.
type Archive struct {
	// The runner to use, eg. "js".
	Type string `json:"type"`

	// Options to use.
	Options Options `json:"options"`

	// TODO: rewrite the encoding, decoding of json to use another type with only the fields it
	// needs in order to remove Filename and Pwd from this
	// Filename and contents of the main file being executed.
	Filename    string   `json:"filename"` // only for json
	FilenameURL *url.URL `json:"-"`
	Data        []byte   `json:"-"`

	// Working directory for resolving relative paths.
	Pwd    string   `json:"pwd"` // only for json
	PwdURL *url.URL `json:"-"`

	Filesystems map[string]afero.Fs `json:"-"`

	// Environment variables
	Env map[string]string `json:"env"`

	CompatibilityMode string `json:"compatibilityMode"`

	K6Version string `json:"k6version"`
	Goos      string `json:"goos"`
}

func (arc *Archive) getFs(name string) afero.Fs {
	fs, ok := arc.Filesystems[name]
	if !ok {
		fs = afero.NewMemMapFs()
		if name == "file" {
			fs = newNormalizedFs(fs)
		}
		arc.Filesystems[name] = fs
	}

	return fs
}

// CleanUpWrongMetadataJSON fixes issues with the metadata.json contents before
// they are unmarshalled in the Archive struct.
//
// Currently, the only fix this function performs is the discarding of the
// derived `execution` config value in the consolidated options that was wrongly
// saved by k6 in the archive metadata.json files until commit
// 83193f8a96e06a190325b838b2cc451119d6b836. This basically means k6 v0.24.0 and
// surrounding master commits. We filter these out by the value of the k6version
// property, saved in the metadata.json since the previous to the above commit.
func CleanUpWrongMetadataJSON(data []byte) ([]byte, error) {
	var tmpArc map[string]interface{}
	if err := json.Unmarshal(data, &tmpArc); err != nil {
		return nil, err
	}

	k6Version := ""
	if k6RawVersion, ok := tmpArc["k6version"]; ok {
		if k6Version, ok = k6RawVersion.(string); !ok {
			return nil, fmt.Errorf("k6version is present in the archive metadata, but it's not a string")
		}
	}

	// TODO: semantically parse the k6version and compare it with the current
	// one, log a warning if the current k6 version in lib/consts is lower than
	// the k6 version that generated the archive.

	if k6Version != "" && k6Version != "0.24.0" {
		return data, nil
	}

	if rawOptions, ok := tmpArc["options"]; !ok {
		return nil, fmt.Errorf("missing options key in the archive metadata.json")
	} else if options, ok := rawOptions.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("wrong options type in metadata.json")
	} else if _, hasExecution := options["execution"]; !hasExecution {
		return data, nil // no need to fix anything
	} else {
		delete(options, "execution")
		tmpArc["options"] = options
	}

	return json.Marshal(tmpArc)
}

func (arc *Archive) loadMetadataJSON(data []byte) error {
	data, err := CleanUpWrongMetadataJSON(data)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(data, &arc); err != nil {
		return err
	}
	// Path separator normalization for older archives (<=0.20.0)
	if arc.K6Version == "" {
		arc.Filename = NormalizeAndAnonymizePath(arc.Filename)
		arc.Pwd = NormalizeAndAnonymizePath(arc.Pwd)
	}
	arc.PwdURL, err = loader.Resolve(&url.URL{Scheme: "file", Path: "/"}, arc.Pwd)
	if err != nil {
		return err
	}
	arc.FilenameURL, err = loader.Resolve(&url.URL{Scheme: "file", Path: "/"}, arc.Filename)
	if err != nil {
		return err
	}

	return nil
}

// ReadArchive reads an archive created by Archive.Write from a reader.
func ReadArchive(in io.Reader) (*Archive, error) {
	r := tar.NewReader(in)
	arc := &Archive{Filesystems: make(map[string]afero.Fs, 2)}
	// initialize both fses
	_ = arc.getFs("https")
	_ = arc.getFs("file")
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
			if err = arc.loadMetadataJSON(data); err != nil {
				return nil, err
			}
			continue
		case "data":
			arc.Data = data
			continue
		}

		// Path separator normalization for older archives (<=0.20.0)
		normPath := NormalizeAndAnonymizePath(hdr.Name)
		idx := strings.IndexRune(normPath, '/')
		if idx == -1 {
			continue
		}
		pfx := normPath[:idx]
		name := normPath[idx:]

		switch pfx {
		case "files", "scripts": // old archives
			// in old archives (pre 0.25.0) names without "_" at the beginning were  https, the ones with "_" are local files
			pfx = "https"
			if len(name) >= 2 && name[0:2] == "/_" {
				pfx = "file"
				name = name[2:]
			}
			fallthrough
		case "https", "file":
			fs := arc.getFs(pfx)
			name = filepath.FromSlash(name)
			err = afero.WriteFile(fs, name, data, os.FileMode(hdr.Mode))
			if err != nil {
				return nil, err
			}
			err = fs.Chtimes(name, hdr.AccessTime, hdr.ModTime)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown file prefix `%s` for file `%s`", pfx, normPath)
		}
	}
	scheme, pathOnFs := getURLPathOnFs(arc.FilenameURL)
	var err error
	pathOnFs, err = url.PathUnescape(pathOnFs)
	if err != nil {
		return nil, err
	}
	err = afero.WriteFile(arc.getFs(scheme), pathOnFs, arc.Data, 0644) // TODO fix the mode ?
	if err != nil {
		return nil, err
	}

	return arc, nil
}

func normalizeAndAnonymizeURL(u *url.URL) {
	if u.Scheme == "file" {
		u.Path = NormalizeAndAnonymizePath(u.Path)
	}
}

func getURLPathOnFs(u *url.URL) (scheme string, pathOnFs string) {
	scheme = "https"
	switch {
	case u.Opaque != "":
		return scheme, "/" + u.Opaque
	case u.Scheme == "":
		return scheme, path.Clean(u.String()[len("//"):])
	default:
		scheme = u.Scheme
	}
	return scheme, path.Clean(u.String()[len(u.Scheme)+len(":/"):])
}

func getURLtoString(u *url.URL) string {
	if u.Opaque == "" && u.Scheme == "" {
		return u.String()[len("//"):] // https url without a scheme
	}
	return u.String()
}

// Write serialises the archive to a writer.
//
// The format should be treated as opaque; currently it is simply a TAR rollup, but this may
// change. If it does change, ReadArchive must be able to handle all previous formats as well as
// the current one.
func (arc *Archive) Write(out io.Writer) error {
	w := tar.NewWriter(out)

	now := time.Now()
	metaArc := *arc
	normalizeAndAnonymizeURL(metaArc.FilenameURL)
	normalizeAndAnonymizeURL(metaArc.PwdURL)
	metaArc.Filename = getURLtoString(metaArc.FilenameURL)
	metaArc.Pwd = getURLtoString(metaArc.PwdURL)
	var actualDataPath, err = url.PathUnescape(path.Join(getURLPathOnFs(metaArc.FilenameURL)))
	if err != nil {
		return err
	}
	var madeLinkToData bool
	metadata, err := metaArc.json()
	if err != nil {
		return err
	}
	_ = w.WriteHeader(&tar.Header{
		Name:     "metadata.json",
		Mode:     0644,
		Size:     int64(len(metadata)),
		ModTime:  now,
		Typeflag: tar.TypeReg,
	})
	if _, err = w.Write(metadata); err != nil {
		return err
	}

	_ = w.WriteHeader(&tar.Header{
		Name:     "data",
		Mode:     0644,
		Size:     int64(len(arc.Data)),
		ModTime:  now,
		Typeflag: tar.TypeReg,
	})
	if _, err = w.Write(arc.Data); err != nil {
		return err
	}
	for _, name := range [...]string{"file", "https"} {
		filesystem, ok := arc.Filesystems[name]
		if !ok {
			continue
		}
		if cachedfs, ok := filesystem.(fsext.CacheOnReadFs); ok {
			filesystem = cachedfs.GetCachingFs()
		}

		// A couple of things going on here:
		// - You can't just create file entries, you need to create directory entries too.
		//   Figure out which directories are in use here.
		// - We want archives to be comparable by hash, which means the entries need to be written
		//   in the same order every time. Go maps are shuffled, so we need to sort lists of keys.
		// - We don't want to leak private information (eg. usernames) in archives, so make sure to
		//   anonymize paths before stuffing them in a shareable archive.
		foundDirs := make(map[string]bool)
		paths := make([]string, 0, 10)
		infos := make(map[string]os.FileInfo) // ... fix this ?
		files := make(map[string][]byte)

		walkFunc := filepath.WalkFunc(func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			normalizedPath := NormalizeAndAnonymizePath(filePath)

			infos[normalizedPath] = info
			if info.IsDir() {
				foundDirs[normalizedPath] = true
				return nil
			}

			paths = append(paths, normalizedPath)
			files[normalizedPath], err = afero.ReadFile(filesystem, filePath)
			return err
		})

		if err = fsext.Walk(filesystem, afero.FilePathSeparator, walkFunc); err != nil {
			return err
		}
		if len(files) == 0 {
			continue // we don't need to write anything for this fs, if this is not done the root will be written
		}
		dirs := make([]string, 0, len(foundDirs))
		for dirpath := range foundDirs {
			dirs = append(dirs, dirpath)
		}
		sort.Strings(paths)
		sort.Strings(dirs)

		for _, dirPath := range dirs {
			_ = w.WriteHeader(&tar.Header{
				Name:       path.Clean(path.Join(name, dirPath)),
				Mode:       0755, // MemMapFs is buggy
				AccessTime: now,  // MemMapFs is buggy
				ChangeTime: now,  // MemMapFs is buggy
				ModTime:    now,  // MemMapFs is buggy
				Typeflag:   tar.TypeDir,
			})
		}

		for _, filePath := range paths {
			var fullFilePath = path.Clean(path.Join(name, filePath))
			// we either have opaque
			if fullFilePath == actualDataPath {
				madeLinkToData = true
				err = w.WriteHeader(&tar.Header{
					Name:     fullFilePath,
					Size:     0,
					Typeflag: tar.TypeLink,
					Linkname: "data",
				})
			} else {
				err = w.WriteHeader(&tar.Header{
					Name:       fullFilePath,
					Mode:       0644, // MemMapFs is buggy
					Size:       int64(len(files[filePath])),
					AccessTime: infos[filePath].ModTime(),
					ChangeTime: infos[filePath].ModTime(),
					ModTime:    infos[filePath].ModTime(),
					Typeflag:   tar.TypeReg,
				})
				if err == nil {
					_, err = w.Write(files[filePath])
				}
			}
			if err != nil {
				return err
			}
		}
	}
	if !madeLinkToData {
		// This should never happen we should always link to `data` from inside the file/https directories
		return fmt.Errorf("archive creation failed because the main script wasn't present in the cached filesystem")
	}

	return w.Close()
}

func (arc *Archive) json() ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	// this prevents <, >, and & from being escaped in JSON strings
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(arc); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
