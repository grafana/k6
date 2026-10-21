package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Uploader uploads a file to a remote store and returns the UUID the store
// assigned to it, if any.
type Uploader interface {
	Upload(ctx context.Context, path string, capturedAt time.Time, data io.Reader) (uuid string, err error)
}

// AsyncPersister queues files and uploads them in the background, one at a time
// and in the order they were queued, so that Persist doesn't wait for the upload.
// A file is dropped with a warning when the queue is full or when Flush has been
// called. Upload failures are logged and don't fail Persist.
type AsyncPersister struct {
	uploader      Uploader
	logger        logrus.FieldLogger
	uploadTimeout time.Duration

	mu     sync.Mutex
	closed bool
	queue  chan asyncUpload
	done   chan struct{}
}

type asyncUpload struct {
	path       string
	capturedAt time.Time
	data       []byte
}

// NewAsyncPersister creates an AsyncPersister that queues up to queueSize files
// and gives each upload up to uploadTimeout to finish.
func NewAsyncPersister(
	uploader Uploader, logger logrus.FieldLogger, queueSize int, uploadTimeout time.Duration,
) *AsyncPersister {
	a := &AsyncPersister{
		uploader:      uploader,
		logger:        logger,
		uploadTimeout: uploadTimeout,
		queue:         make(chan asyncUpload, queueSize),
		done:          make(chan struct{}),
	}
	go a.run()

	return a
}

// Persist queues data to be uploaded and returns without waiting for the upload.
func (a *AsyncPersister) Persist(_ context.Context, path string, data io.Reader) error {
	capturedAt := time.Now()

	// Copy the data, since the caller may reuse or change it before the upload.
	bb, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("reading file data: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		a.logger.Warnf("dropping screenshot %q: uploads have stopped", path)
		return nil
	}
	select {
	case a.queue <- asyncUpload{path: path, capturedAt: capturedAt, data: bb}:
	default:
		a.logger.Warnf("dropping screenshot %q: upload queue is full", path)
	}

	return nil
}

// Flush stops accepting new files and waits until the queued ones have been
// uploaded, or until ctx is done.
func (a *AsyncPersister) Flush(ctx context.Context) error {
	a.mu.Lock()
	if !a.closed {
		a.closed = true
		close(a.queue)
	}
	a.mu.Unlock()

	select {
	case <-a.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%d queued screenshots were not uploaded: %w", len(a.queue), ctx.Err())
	}
}

func (a *AsyncPersister) run() {
	defer close(a.done)

	for u := range a.queue {
		a.upload(u)
	}
}

func (a *AsyncPersister) upload(u asyncUpload) {
	ctx, cancel := context.WithTimeout(context.Background(), a.uploadTimeout)
	defer cancel()

	uuid, err := a.uploader.Upload(ctx, u.path, u.capturedAt, bytes.NewReader(u.data))
	if err != nil {
		a.logger.Warnf("uploading screenshot %q: %v", u.path, err)
		return
	}
	logUploadedUUID(a.logger, u.capturedAt, uuid)
}
