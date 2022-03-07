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
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/types"
)

// Panic if the given error is not nil.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: refactor the CLI config so these functions aren't needed - they
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
	// TODO: use types.ParseExtendedDuration? not sure we should support
	// unitless durations (i.e. milliseconds) here...
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

func exactArgsWithMsg(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("accepts %d arg(s), received %d: %s", n, len(args), msg)
		}
		return nil
	}
}

func printToStdout(gs *globalState, s string) {
	if _, err := fmt.Fprint(gs.stdOut, s); err != nil {
		gs.logger.Errorf("could not print '%s' to stdout: %s", s, err.Error())
	}
}

// Trap Interrupts, SIGINTs and SIGTERMs and call the given.
func handleTestAbortSignals(gs *globalState, firstHandler, secondHandler func(os.Signal)) (stop func()) {
	sigC := make(chan os.Signal, 2)
	done := make(chan struct{})
	gs.signalNotify(sigC, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigC:
			firstHandler(sig)
		case <-done:
			return
		}

		select {
		case sig := <-sigC:
			if secondHandler != nil {
				secondHandler(sig)
			}
			// If we get a second signal, we immediately exit, so something like
			// https://github.com/k6io/k6/issues/971 never happens again
			gs.osExit(int(exitcodes.ExternalAbort))
		case <-done:
			return
		}
	}()

	return func() {
		close(done)
		gs.signalStop(sigC)
	}
}
