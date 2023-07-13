package log

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

type nopCloser struct {
	io.Writer
	closed chan struct{}
}

func (nc *nopCloser) Close() error {
	nc.closed <- struct{}{}
	return nil
}

func TestFileHookFromConfigLine(t *testing.T) {
	t.Parallel()

	tests := [...]struct {
		line       string
		err        bool
		errMessage string
		res        fileHook
	}{
		{
			line: "file",
			err:  true,
			res: fileHook{
				levels: logrus.AllLevels,
			},
		},
		{
			line: "file=/k6.log,level=info",
			err:  false,
			res: fileHook{
				path:   "/k6.log",
				levels: logrus.AllLevels[:5],
			},
		},
		{
			line: "file=/a/c/",
			err:  true,
		},
		{
			line:       "file=,level=info",
			err:        true,
			errMessage: "filepath must not be empty",
		},
		{
			line: "file=/tmp/k6.log,level=tea",
			err:  true,
		},
		{
			line: "file=/tmp/k6.log,unknown",
			err:  true,
		},
		{
			line: "file=/tmp/k6.log,level=",
			err:  true,
		},
		{
			line: "file=/tmp/k6.log,level=,",
			err:  true,
		},
		{
			line:       "file=/tmp/k6.log,unknown=something",
			err:        true,
			errMessage: "unknown logfile config key unknown",
		},
		{
			line:       "unknown=something",
			err:        true,
			errMessage: "logfile configuration should be in the form `file=path-to-local-file` but is `unknown=something`",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.line, func(t *testing.T) {
			t.Parallel()

			getCwd := func() (string, error) {
				return "/", nil
			}

			res, err := FileHookFromConfigLine(
				fsext.NewMemMapFs(), getCwd, logrus.New(), test.line)

			if test.err {
				require.Error(t, err)

				if test.errMessage != "" {
					require.Equal(t, test.errMessage, err.Error())
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, res.(*fileHook).w) //nolint:forcetypeassert
		})
	}
}

func TestFileHookFire(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	nc := &nopCloser{
		Writer: &buffer,
		closed: make(chan struct{}),
	}

	hook := &fileHook{
		loglines: make(chan []byte),
		w:        nc,
		bw:       bufio.NewWriter(nc),
		levels:   logrus.AllLevels,
	}

	ctx, cancel := context.WithCancel(context.Background())

	go hook.Listen(ctx)

	logger := logrus.New()
	logger.AddHook(hook)
	logger.SetOutput(io.Discard)

	logger.Info("example log line")

	time.Sleep(10 * time.Millisecond)

	cancel()
	<-nc.closed

	assert.Contains(t, buffer.String(), "example log line")
}
