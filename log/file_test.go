/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package log

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			line: fmt.Sprintf("file=%s/k6.log,level=info", os.TempDir()),
			err:  false,
			res: fileHook{
				path:   fmt.Sprintf("%s/k6.log", os.TempDir()),
				levels: logrus.AllLevels[:5],
			},
		},
		{
			line: "file=./",
			err:  true,
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

			res, err := FileHookFromConfigLine(context.Background(), logrus.New(), test.line)

			if test.err {
				require.Error(t, err)

				if test.errMessage != "" {
					require.Equal(t, test.errMessage, err.Error())
				}

				return
			}

			require.NoError(t, err)
			assert.NotNil(t, res.(*fileHook).w)
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

	hook.loglines = hook.loop(ctx)

	logger := logrus.New()
	logger.AddHook(hook)
	logger.SetOutput(io.Discard)

	logger.Info("example log line")

	time.Sleep(10 * time.Millisecond)

	cancel()
	<-nc.closed

	assert.Contains(t, buffer.String(), "example log line")
}
