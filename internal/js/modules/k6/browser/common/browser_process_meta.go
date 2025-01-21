package common

import (
	"os"

	"go.k6.io/k6/internal/js/modules/k6/browser/storage"
)

const (
	unknownProcessPid = -1
)

// browserProcessMeta handles the metadata associated with
// a browser process, especifically, the OS process handle
// and the associated browser data directory.
type browserProcessMeta interface {
	Pid() int
	Cleanup() error
}

// localBrowserProcessMeta holds the metadata for local
// browser process.
type localBrowserProcessMeta struct {
	process     *os.Process //nolint:forbidigo
	userDataDir *storage.Dir
}

// newLocalBrowserProcessMeta returns a new BrowserProcessMeta
// for the given OS process and storage directory.
func newLocalBrowserProcessMeta(
	process *os.Process, userDataDir *storage.Dir, //nolint:forbidigo
) *localBrowserProcessMeta {
	return &localBrowserProcessMeta{
		process,
		userDataDir,
	}
}

// Pid returns the Pid for the local browser process.
func (l *localBrowserProcessMeta) Pid() int {
	return l.process.Pid
}

// Cleanup cleans the local user data directory associated
// with the local browser process.
func (l *localBrowserProcessMeta) Cleanup() error {
	return l.userDataDir.Cleanup() //nolint:wrapcheck
}

// remoteBrowserProcessMeta is a placeholder for a
// remote browser process metadata.
type remoteBrowserProcessMeta struct{}

// newRemoteBrowserProcessMeta returns a new BrowserProcessMeta
// which acts as a placeholder for a remote browser process data.
func newRemoteBrowserProcessMeta() *remoteBrowserProcessMeta {
	return &remoteBrowserProcessMeta{}
}

// Pid returns -1 as the remote browser process is unknown.
func (r *remoteBrowserProcessMeta) Pid() int {
	return unknownProcessPid
}

// Cleanup does nothing and returns nil, as there is no
// access to the remote browser's user data directory.
func (r *remoteBrowserProcessMeta) Cleanup() error {
	// Nothing to do.
	return nil
}
