package lib

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/lib/fsext"
)

var (
	volumeRE = regexp.MustCompile(`^[/\\]?([a-zA-Z]):(.*)`)
	// matches a shared folder in Windows before backslack replacement. i.e \\VMBOXSVR\k6\script.js
	sharedRE  = regexp.MustCompile(`^\\\\([^\\]+)`)
	homeDirRE = regexp.MustCompile(`(?i)^(/[a-zA-Z])?/(Users|home|Documents and Settings)/(?:[^/]+)`)
)

// NormalizeAndAnonymizePath Normalizes (to use a / path separator) and anonymizes a file path,
// by scrubbing usernames from home directories.
func NormalizeAndAnonymizePath(path string) string {
	path = filepath.Clean(path)

	p := volumeRE.ReplaceAllString(path, `/$1$2`)
	p = sharedRE.ReplaceAllString(p, `/nobody`)
	p = strings.ReplaceAll(p, "\\", "/")
	return homeDirRE.ReplaceAllString(p, `$1/$2/nobody`)
}

func newNormalizedFs(fs fsext.Fs) fsext.Fs {
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

	Filesystems map[string]fsext.Fs `json:"-"`

	// Environment variables
	Env map[string]string `json:"env"`

	CompatibilityMode string `json:"compatibilityMode"`

	K6Version string `json:"k6version"`
	Goos      string `json:"goos"`
}

func (arc *Archive) getFs(name string) fsext.Fs {
	fs, ok := arc.Filesystems[name]
	if !ok {
		fs = fsext.NewMemMapFs()
		if name == "file" {
			fs = newNormalizedFs(fs)
		}
		arc.Filesystems[name] = fs
	}

	return fs
}

func (arc *Archive) loadMetadataJSON(data []byte) (err error) {
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
//
//nolint:gocognit
func ReadArchive(in io.Reader) (*Archive, error) {
	r := tar.NewReader(in)
	arc := &Archive{Filesystems: make(map[string]fsext.Fs, 2)}
	// initialize both fses
	_ = arc.getFs("https")
	_ = arc.getFs("file")
	for {
		hdr, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA { //nolint:staticcheck
			continue
		}

		data, err := io.ReadAll(r)
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
			fileSystem := arc.getFs(pfx)
			name = filepath.FromSlash(name)
			if err = fsext.WriteFile(fileSystem, name, data, fs.FileMode(hdr.Mode)); err != nil { //nolint:gosec
				return nil, err
			}
			if err = fileSystem.Chtimes(name, hdr.AccessTime, hdr.ModTime); err != nil {
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
	err = fsext.WriteFile(arc.getFs(scheme), pathOnFs, arc.Data, 0o644) // TODO fix the mode ?
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
//
//nolint:funlen,gocognit
func (arc *Archive) Write(out io.Writer) error {
	w := tar.NewWriter(out)

	now := time.Now()
	metaArc := *arc
	normalizeAndAnonymizeURL(metaArc.FilenameURL)
	normalizeAndAnonymizeURL(metaArc.PwdURL)
	metaArc.Filename = getURLtoString(metaArc.FilenameURL)
	metaArc.Pwd = getURLtoString(metaArc.PwdURL)
	actualDataPath, err := url.PathUnescape(path.Join(getURLPathOnFs(metaArc.FilenameURL)))
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
		Mode:     0o644,
		Size:     int64(len(metadata)),
		ModTime:  now,
		Typeflag: tar.TypeReg,
	})
	if _, err = w.Write(metadata); err != nil {
		return err
	}

	_ = w.WriteHeader(&tar.Header{
		Name:     "data",
		Mode:     0o644,
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
		if cachedfs, ok := filesystem.(fsext.CacheLayerGetter); ok {
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
		infos := make(map[string]fs.FileInfo) // ... fix this ?
		files := make(map[string][]byte)

		walkFunc := filepath.WalkFunc(func(filePath string, info fs.FileInfo, err error) error {
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
			files[normalizedPath], err = fsext.ReadFile(filesystem, filePath)
			return err
		})

		if err = fsext.Walk(filesystem, fsext.FilePathSeparator, walkFunc); err != nil {
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
				Mode:       0o755, // MemMapFs is buggy
				AccessTime: now,   // MemMapFs is buggy
				ChangeTime: now,   // MemMapFs is buggy
				ModTime:    now,   // MemMapFs is buggy
				Typeflag:   tar.TypeDir,
			})
		}

		for _, filePath := range paths {
			fullFilePath := path.Clean(path.Join(name, filePath))
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
					Mode:       0o644, // MemMapFs is buggy
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
