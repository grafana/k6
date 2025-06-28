package fsext

import "io/fs"

// Bridge allows an afero.Fs to implement the Go standard library io/fs.FS.
type IOFSBridge struct {
	FSExt Fs
}

func (b *IOFSBridge) Open(name string) (fs.File, error) {
	f, err := b.FSExt.Open(name)
	if err != nil {
		return nil, err // TODO: wrap
	}
	return f, nil
}
