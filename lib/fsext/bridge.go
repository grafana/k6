package fsext

import "io/fs"

// IOFSBridge allows an afero.Fs to implement the Go standard library io/fs.FS.
type IOFSBridge struct {
	FSExt Fs
}

// NewIOFSBridge returns an IOFSBridge from a Fs
func NewIOFSBridge(fs Fs) fs.FS {
	return &IOFSBridge{
		FSExt: fs,
	}
}

// Open implements fs.Fs Open
func (b *IOFSBridge) Open(name string) (fs.File, error) {
	f, err := b.FSExt.Open(name)
	if err != nil {
		return nil, err // TODO: wrap
	}
	return f, nil
}
