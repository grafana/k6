/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package testutils

import (
	"io"
	"testing"

	"github.com/sirupsen/logrus"
)

// Something that makes the test also be a valid io.Writer, useful for passing it
// as an output for logs and CLI flag help messages...
type testOutput struct{ testing.TB }

func (to testOutput) Write(p []byte) (n int, err error) {
	to.Logf("%s", p)

	return len(p), nil
}

// NewTestOutput returns a simple io.Writer implementation that uses the test's
// logger as an output.
func NewTestOutput(t testing.TB) io.Writer {
	return testOutput{t}
}

// NewLogger Returns new logger that will log to the testing.TB.Logf
func NewLogger(t testing.TB) *logrus.Logger {
	l := logrus.New()
	logrus.SetOutput(NewTestOutput(t))

	return l
}
