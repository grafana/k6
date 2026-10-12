package common

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// fakeUUIDPersister implements ScreenshotPersister returning a
// caller-controlled UUID to verify that the screenshotter logs it.
type fakeUUIDPersister struct {
	uuid string
}

func (f *fakeUUIDPersister) Persist(_ context.Context, _ string, _ io.Reader) error {
	_, err := f.PersistWithResult(context.Background(), "", nil)
	return err
}

func (f *fakeUUIDPersister) PersistWithResult(
	_ context.Context, _ string, _ io.Reader,
) (string, error) {
	return f.uuid, nil
}


// screenshotUUIDSession is a CDP session double that satisfies the minimum
// surface the screenshotter needs to complete a screenshot capture.
type screenshotUUIDSession struct {
	session
}

func (s *screenshotUUIDSession) Execute(_ context.Context, _ string, _, _ any) error {
	return nil
}

func newCapturingLogrus(t *testing.T, buf *bytes.Buffer) *logrus.Logger {
	t.Helper()
	l := logrus.New()
	l.SetOutput(buf)
	l.SetLevel(logrus.InfoLevel)
	return l
}

func TestScreenshotterLogsUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		uuid     string
		wantLog  bool
		wantLine string
	}{
		{
			name:     "uuid_is_logged",
			uuid:     "7fc56f68-1e6b-4b64-8f3f-3b0d1a3c9c31",
			wantLog:  true,
			wantLine: "screenshot:7fc56f68-1e6b-4b64-8f3f-3b0d1a3c9c31",
		},
		{
			name:    "no_uuid_no_log",
			uuid:    "",
			wantLog: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logrusLogger := newCapturingLogrus(t, &buf)
			logger := log.New(logrusLogger, "")

			s, cancel := newScreenshotter(
				context.Background(),
				0,
				&fakeUUIDPersister{uuid: tt.uuid},
				logger,
			)
			defer cancel()

			sess := &screenshotUUIDSession{session: &Session{}}
			doc := &Rect{Width: 100, Height: 100}
			_, err := s.screenshot(sess, doc, nil, ImageFormatPNG, false, 0, "some/file.png")
			require.NoError(t, err)

			out := buf.String()
			if tt.wantLog {
				assert.Contains(t, out, tt.wantLine)
			} else {
				assert.NotContains(t, out, "screenshot:")
			}
		})
	}
}
