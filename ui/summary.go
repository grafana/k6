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

package ui

import (
	"fmt"
	"io"
	"sort"

	"github.com/fatih/color"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

const (
	GroupPrefix   = "█"
	DetailsPrefix = "↪"

	SuccMark = "✓"
	FailMark = "✗"
)

var (
	SuccColor = color.New(color.FgGreen)
	FailColor = color.New(color.FgRed)
)

// SummaryData represents data passed to Summarize.
type SummaryData struct {
	Opts    lib.Options
	Root    *lib.Group
	Metrics map[string]*stats.Metric
}

func SummarizeCheck(w io.Writer, tty bool, indent string, check *lib.Check) {
	mark := SuccMark
	color := SuccColor
	if check.Fails > 0 {
		mark = FailMark
		color = FailColor
	}
	_, _ = color.Fprintf(w, "%s%s %s\n", indent, mark, check.Name)
	if check.Fails > 0 {
		_, _ = color.Fprintf(w, "%s %s  %d%% — %s %d / %s %d\n",
			indent, DetailsPrefix,
			int(100*(float64(check.Passes)/float64(check.Fails+check.Passes))),
			SuccMark, check.Passes, FailMark, check.Fails,
		)
	}
}

func SummarizeGroup(w io.Writer, tty bool, indent string, group *lib.Group) {
	if group.Name != "" {
		_, _ = fmt.Fprintf(w, "%s%s %s\n\n", indent, GroupPrefix, group.Name)
	}

	var checkNames []string
	for _, check := range group.Checks {
		checkNames = append(checkNames, check.Name)
	}
	sort.Strings(checkNames)
	for _, name := range checkNames {
		SummarizeCheck(w, tty, indent+"  ", group.Checks[name])
	}
	if len(checkNames) > 0 {
		fmt.Fprintf(w, "\n")
	}

	var groupNames []string
	for _, grp := range group.Groups {
		groupNames = append(groupNames, grp.Name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		SummarizeGroup(w, tty, indent+"  ", group.Groups[name])
	}
}

// Summarizes a dataset and returns whether the test run was considered a success.
func Summarize(w io.Writer, tty bool, indent string, data SummaryData) {
	SummarizeGroup(w, tty, indent, data.Root)
}
