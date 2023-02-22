package fsext

import (
	"io/fs"
	"path/filepath"
	"sort"

	"github.com/spf13/afero"
)

// Walk implements afero.Walk, but in a way that it doesn't loop to infinity and doesn't have
// problems if a given path part looks like a windows volume name
func Walk(fs afero.Fs, root string, walkFn filepath.WalkFunc) error {
	info, err := fs.Stat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walk(fs, root, info, walkFn)
}

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entries.
// adapted from https://github.com/spf13/afero/blob/master/path.go#L27
func readDirNames(fs afero.Fs, dirname string) ([]string, error) {
	f, err := fs.Open(dirname)
	if err != nil {
		return nil, err
	}
	infos, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	err = f.Close()

	if err != nil {
		return nil, err
	}

	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name()
	}
	sort.Strings(names)
	return names, nil
}

// walk recursively descends path, calling walkFn
// adapted from https://github.com/spf13/afero/blob/master/path.go#L27
func walk(fileSystem afero.Fs, path string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}

	if !info.IsDir() {
		return nil
	}

	names, err := readDirNames(fileSystem, path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, name := range names {
		filename := JoinFilePath(path, name)
		fileInfo, err := fileSystem.Stat(filename)
		if err != nil {
			if err = walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return err
			}
		} else {
			err = walk(fileSystem, filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return err
				}
			}
		}
	}
	return nil
}
