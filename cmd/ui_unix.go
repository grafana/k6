// +build darwin dragonfly freebsd linux netbsd openbsd

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

package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// GetTermSize returns the current terminal window size on Windows,
// but is a no-op on all other platforms.
func GetTermSize(fd, termWidth int) (width, height int, err error) {
	return termWidth, 0, nil
}

// NotifyWindowResize listens for SIGWINCH (terminal window size changes)
// on *nix platforms, and is a no-op on Windows.
func NotifyWindowResize() <-chan os.Signal {
	sch := make(chan os.Signal, 1)
	signal.Notify(sch, os.Signal(syscall.SIGWINCH))
	return sch
}
