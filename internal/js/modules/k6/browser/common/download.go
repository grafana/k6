package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"

	"go.k6.io/k6/internal/js/modules/k6/browser/storage"
)

// Download represents a file download initiated by a browser page.
type Download struct {
	url               string
	suggestedFilename string
	guid              string

	// page is the page that initiated the download.
	page *Page

	// downloadsPath is the base directory where downloads are saved.
	downloadsPath string

	mu       sync.Mutex
	finished chan struct{}
	err      string // empty means success
}

// newDownload creates a new Download.
func newDownload(page *Page, guid, url, suggestedFilename, downloadsPath string) *Download {
	return &Download{
		page:              page,
		guid:              guid,
		url:               url,
		suggestedFilename: suggestedFilename,
		downloadsPath:     downloadsPath,
		finished:          make(chan struct{}),
	}
}

// URL returns the URL of the download.
func (d *Download) URL() string {
	return d.url
}

// SuggestedFilename returns the suggested file name for the download.
func (d *Download) SuggestedFilename() string {
	return d.suggestedFilename
}

// Path returns the path to the downloaded file once it completes.
// Returns an error if the download has not finished or was canceled.
func (d *Download) Path() (string, error) {
	<-d.finished

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.err != "" {
		return "", fmt.Errorf("download failed: %s", d.err)
	}

	return filepath.Join(d.downloadsPath, d.guid), nil
}

// Failure returns the error message if the download failed or was canceled.
// Returns empty string if the download succeeded.
func (d *Download) Failure() string {
	<-d.finished

	d.mu.Lock()
	defer d.mu.Unlock()

	return d.err
}

// SaveAs copies the downloaded file to the given path.
func (d *Download) SaveAs(path string) error {
	<-d.finished

	d.mu.Lock()
	errMsg := d.err
	d.mu.Unlock()

	if errMsg != "" {
		return fmt.Errorf("download failed: %s", errMsg)
	}

	src := filepath.Join(d.downloadsPath, d.guid)

	in, err := os.Open(src) //nolint:forbidigo,gosec
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer in.Close() //nolint:errcheck

	persister := &storage.LocalFilePersister{}
	if err := persister.Persist(context.Background(), path, in); err != nil {
		return fmt.Errorf("saving download: %w", err)
	}

	return nil
}

// Cancel cancels the download by sending a Browser.cancelDownload CDP command.
// This is a no-op if the download has already finished.
func (d *Download) Cancel() error {
	d.mu.Lock()
	select {
	case <-d.finished:
		d.mu.Unlock()
		return nil // already done
	default:
		d.mu.Unlock()
	}

	action := cdpbrowser.
		CancelDownload(d.guid).
		WithBrowserContextID(d.page.browserCtx.id)
	if err := action.Do(cdp.WithExecutor(d.page.ctx, d.page.browserCtx.browser.conn)); err != nil {
		return fmt.Errorf("canceling download: %w", err)
	}

	return nil
}

// Page returns the page that initiated the download.
func (d *Download) Page() *Page {
	return d.page
}

// finish marks the download as finished with an optional error.
func (d *Download) finish(errMsg string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.finished:
		return // already finished
	default:
		d.err = errMsg
		close(d.finished)
	}
}
