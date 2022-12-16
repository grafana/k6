package console

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// Writer syncs writes with a mutex and, if the output is a TTY, clears before
// newlines.
type Writer struct {
	RawOut *os.File
	Mutex  *sync.Mutex
	Writer io.Writer
	IsTTY  bool

	// Used for flicker-free persistent objects like the progressbars
	PersistentText func()
}

func (w *Writer) Write(p []byte) (n int, err error) {
	origLen := len(p)
	if w.IsTTY {
		// Add a TTY code to erase till the end of line with each new line
		// TODO: check how cross-platform this is...
		p = bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\x1b', '[', '0', 'K', '\n'})
	}

	w.Mutex.Lock()
	n, err = w.Writer.Write(p)
	if w.PersistentText != nil {
		w.PersistentText()
	}
	w.Mutex.Unlock()

	if err != nil && n < origLen {
		return n, err
	}
	return origLen, err
}
