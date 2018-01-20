/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"
)

// Panic if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// Silently set an exit code.
type ExitCode struct {
	error
	Code int
}

// A writer that syncs writes with a mutex and, if the output is a TTY, clears before newlines.
type consoleWriter struct {
	Writer io.Writer
	IsTTY  bool
	Mutex  *sync.Mutex
}

func (w consoleWriter) Write(p []byte) (n int, err error) {
	if w.IsTTY {
		p = bytes.Replace(p, []byte{'\n'}, []byte{'\x1b', '[', '0', 'K', '\n'}, -1)
	}
	w.Mutex.Lock()
	n, err = w.Writer.Write(p)
	w.Mutex.Unlock()
	return
}

func getNullBool(flags *pflag.FlagSet, key string) null.Bool {
	v, err := flags.GetBool(key)
	if err != nil {
		panic(err)
	}
	return null.NewBool(v, flags.Changed(key))
}

func getNullInt64(flags *pflag.FlagSet, key string) null.Int {
	v, err := flags.GetInt64(key)
	if err != nil {
		panic(err)
	}
	return null.NewInt(v, flags.Changed(key))
}

func getNullDuration(flags *pflag.FlagSet, key string) lib.NullDuration {
	v, err := flags.GetDuration(key)
	if err != nil {
		panic(err)
	}
	return lib.NullDuration{Duration: lib.Duration(v), Valid: flags.Changed(key)}
}

func getNullString(flags *pflag.FlagSet, key string) null.String {
	v, err := flags.GetString(key)
	if err != nil {
		panic(err)
	}
	return null.NewString(v, flags.Changed(key))
}

func exactArgsWithMsg(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d: %s", n, len(args), msg)
		}
		return nil
	}
}
