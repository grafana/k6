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
	"os"
	"sync"

	"github.com/loadimpact/k6/lib/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	null "gopkg.in/guregu/null.v3"
)

// Use these when interacting with fs and writing to terminal, makes a command testable
var defaultFs = afero.NewOsFs()
var defaultWriter io.Writer = os.Stdout

// Panic if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// ExitCode wraps the error with an exit code.
// Hint is used to show details information about underlying error.
type ExitCode struct {
	error
	Code int
	Hint string
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

//TODO: refactor the CLI config so these functions aren't needed - they
// can mask errors by failing only at runtime, not at compile time
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

func getNullDuration(flags *pflag.FlagSet, key string) types.NullDuration {
	v, err := flags.GetDuration(key)
	if err != nil {
		panic(err)
	}
	return types.NullDuration{Duration: types.Duration(v), Valid: flags.Changed(key)}
}

func getNullString(flags *pflag.FlagSet, key string) null.String {
	v, err := flags.GetString(key)
	if err != nil {
		panic(err)
	}
	return null.NewString(v, flags.Changed(key))
}

func getStringSlice(flags *pflag.FlagSet, key string) []string {
	v, err := flags.GetStringSlice(key)
	if err != nil {
		panic(err)
	}
	return v
}

func exactArgsWithMsg(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d: %s", n, len(args), msg)
		}
		return nil
	}
}
