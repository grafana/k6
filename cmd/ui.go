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

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	"gopkg.in/yaml.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/output"
	"go.k6.io/k6/ui/pb"
)

const (
	// Max length of left-side progress bar text before trimming is forced
	maxLeftLength = 30
	// Amount of padding in chars between rendered progress
	// bar text and right-side terminal window edge.
	termPadding      = 1
	defaultTermWidth = 80
)

// A writer that syncs writes with a mutex and, if the output is a TTY, clears before newlines.
type consoleWriter struct {
	Writer io.Writer
	IsTTY  bool
	Mutex  *sync.Mutex

	// Used for flicker-free persistent objects like the progressbars
	PersistentText func()
}

func (w *consoleWriter) Write(p []byte) (n int, err error) {
	origLen := len(p)
	if w.IsTTY {
		// Add a TTY code to erase till the end of line with each new line
		// TODO: check how cross-platform this is...
		p = bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\x1b', '[', '0', 'K', '\n'})
	}

	w.Mutex.Lock()
	n, err = w.Writer.Write(p)
	if w.PersistentText != nil {
		w.PersistentText()
	}
	w.Mutex.Unlock()

	if err != nil && n < origLen {
		return n, err
	}
	return origLen, err
}

// getColor returns the requested color, or an uncolored object, depending on
// the value of noColor. The explicit EnableColor() and DisableColor() are
// needed because the library checks os.Stdout itself otherwise...
func getColor(noColor bool, attributes ...color.Attribute) *color.Color {
	if noColor {
		c := color.New()
		c.DisableColor()
		return c
	}

	c := color.New(attributes...)
	c.EnableColor()
	return c
}

func getBanner(noColor bool) string {
	c := getColor(noColor, color.FgCyan)
	return c.Sprint(consts.Banner())
}

func printBar(bar *pb.ProgressBar, globalFlags *commandFlags) {
	if globalFlags.quiet {
		return
	}
	end := "\n"
	// TODO: refactor widthDelta away? make the progressbar rendering a bit more
	// stateless... basically first render the left and right parts, so we know
	// how long the longest line is, and how much space we have for the progress
	widthDelta := -defaultTermWidth
	if stdout.IsTTY {
		// If we're in a TTY, instead of printing the bar and going to the next
		// line, erase everything till the end of the line and return to the
		// start, so that the next print will overwrite the same line.
		//
		// TODO: check for cross platform support
		end = "\x1b[0K\r"
		widthDelta = 0
	}
	rendered := bar.Render(0, widthDelta)
	// Only output the left and middle part of the progress bar
	fprintf(stdout, "%s%s", rendered.String(), end)
}

func modifyAndPrintBar(bar *pb.ProgressBar, globalFlags *commandFlags, options ...pb.ProgressBarOption) {
	bar.Modify(options...)
	printBar(bar, globalFlags)
}

// Print execution description for both cloud and local execution.
// TODO: Clean this up as part of #1499 or #1427
func printExecutionDescription(
	execution, filename, outputOverride string, conf Config, et *lib.ExecutionTuple,
	execPlan []lib.ExecutionStep, outputs []output.Output, noColor bool,
) {
	valueColor := getColor(noColor, color.FgCyan)

	fprintf(stdout, "  execution: %s\n", valueColor.Sprint(execution))
	fprintf(stdout, "     script: %s\n", valueColor.Sprint(filename))

	var outputDescriptions []string
	switch {
	case outputOverride != "":
		outputDescriptions = []string{outputOverride}
	case len(outputs) == 0:
		outputDescriptions = []string{"-"}
	default:
		for _, out := range outputs {
			outputDescriptions = append(outputDescriptions, out.Description())
		}
	}

	fprintf(stdout, "     output: %s\n", valueColor.Sprint(strings.Join(outputDescriptions, ", ")))
	fprintf(stdout, "\n")

	maxDuration, _ := lib.GetEndOffset(execPlan)
	executorConfigs := conf.Scenarios.GetSortedConfigs()

	scenarioDesc := "1 scenario"
	if len(executorConfigs) > 1 {
		scenarioDesc = fmt.Sprintf("%d scenarios", len(executorConfigs))
	}

	fprintf(stdout, "  scenarios: %s\n", valueColor.Sprintf(
		"(%.2f%%) %s, %d max VUs, %s max duration (incl. graceful stop):",
		conf.ExecutionSegment.FloatLength()*100, scenarioDesc,
		lib.GetMaxPossibleVUs(execPlan), maxDuration.Round(100*time.Millisecond)),
	)
	for _, ec := range executorConfigs {
		fprintf(stdout, "           * %s: %s\n",
			ec.GetName(), ec.GetDescription(et))
	}
	fprintf(stdout, "\n")
}

//nolint: funlen
func renderMultipleBars(
	isTTY, goBack bool, maxLeft, termWidth, widthDelta int, pbs []*pb.ProgressBar, globalFlags *commandFlags,
) (string, int) {
	lineEnd := "\n"
	if isTTY {
		// TODO: check for cross platform support
		lineEnd = "\x1b[K\n" // erase till end of line
	}

	var (
		// Amount of times line lengths exceed termWidth.
		// Needed to factor into the amount of lines to jump
		// back with [A and avoid scrollback issues.
		lineBreaks  int
		longestLine int
		// Maximum length of each right side column except last,
		// used to calculate the padding between columns.
		maxRColumnLen = make([]int, 2)
		pbsCount      = len(pbs)
		rendered      = make([]pb.ProgressBarRender, pbsCount)
		result        = make([]string, pbsCount+2)
	)

	result[0] = lineEnd // start with an empty line

	// First pass to render all progressbars and get the maximum
	// lengths of right-side columns.
	for i, pb := range pbs {
		rend := pb.Render(maxLeft, widthDelta)
		for i := range rend.Right {
			// Skip last column, since there's nothing to align after it (yet?).
			if i == len(rend.Right)-1 {
				break
			}
			if len(rend.Right[i]) > maxRColumnLen[i] {
				maxRColumnLen[i] = len(rend.Right[i])
			}
		}
		rendered[i] = rend
	}

	// Second pass to render final output, applying padding where needed
	for i := range rendered {
		rend := rendered[i]
		if rend.Hijack != "" {
			result[i+1] = rend.Hijack + lineEnd
			runeCount := utf8.RuneCountInString(rend.Hijack)
			lineBreaks += (runeCount - termPadding) / termWidth
			continue
		}
		var leftText, rightText string
		leftPadFmt := fmt.Sprintf("%%-%ds", maxLeft)
		leftText = fmt.Sprintf(leftPadFmt, rend.Left)
		for i := range rend.Right {
			rpad := 0
			if len(maxRColumnLen) > i {
				rpad = maxRColumnLen[i]
			}
			rightPadFmt := fmt.Sprintf(" %%-%ds", rpad+1)
			rightText += fmt.Sprintf(rightPadFmt, rend.Right[i])
		}
		// Get visible line length, without ANSI escape sequences (color)
		status := fmt.Sprintf(" %s ", rend.Status())
		line := leftText + status + rend.Progress() + rightText
		lineRuneCount := utf8.RuneCountInString(line)
		if lineRuneCount > longestLine {
			longestLine = lineRuneCount
		}
		lineBreaks += (lineRuneCount - termPadding) / termWidth
		if !globalFlags.noColor {
			rend.Color = true
			status = fmt.Sprintf(" %s ", rend.Status())
			line = fmt.Sprintf(leftPadFmt+"%s%s%s",
				rend.Left, status, rend.Progress(), rightText)
		}
		result[i+1] = line + lineEnd
	}

	if isTTY && goBack {
		// Clear screen and go back to the beginning
		// TODO: check for cross platform support
		result[pbsCount+1] = fmt.Sprintf("\r\x1b[J\x1b[%dA", pbsCount+lineBreaks+1)
	} else {
		result[pbsCount+1] = ""
	}

	return strings.Join(result, ""), longestLine
}

// TODO: show other information here?
// TODO: add a no-progress option that will disable these
// TODO: don't use global variables...
// nolint:funlen,gocognit
func showProgress(ctx context.Context, pbs []*pb.ProgressBar, logger *logrus.Logger, globalFlags *commandFlags) {
	if globalFlags.quiet {
		return
	}

	var errTermGetSize bool
	termWidth := defaultTermWidth
	if stdoutTTY {
		tw, _, err := term.GetSize(int(os.Stdout.Fd()))
		if !(tw > 0) || err != nil {
			errTermGetSize = true
			logger.WithError(err).Warn("error getting terminal size")
		} else {
			termWidth = tw
		}
	}

	// Get the longest left side string length, to align progress bars
	// horizontally and trim excess text.
	var leftLen int64
	for _, pb := range pbs {
		l := pb.Left()
		leftLen = lib.Max(int64(len(l)), leftLen)
	}
	// Limit to maximum left text length
	maxLeft := int(lib.Min(leftLen, maxLeftLength))

	var progressBarsLastRenderLock sync.Mutex
	var progressBarsLastRender []byte

	printProgressBars := func() {
		progressBarsLastRenderLock.Lock()
		_, _ = stdout.Writer.Write(progressBarsLastRender)
		progressBarsLastRenderLock.Unlock()
	}

	var widthDelta int
	// Default to responsive progress bars when in an interactive terminal
	renderProgressBars := func(goBack bool) {
		barText, longestLine := renderMultipleBars(stdoutTTY, goBack, maxLeft, termWidth, widthDelta, pbs, globalFlags)
		widthDelta = termWidth - longestLine - termPadding
		progressBarsLastRenderLock.Lock()
		progressBarsLastRender = []byte(barText)
		progressBarsLastRenderLock.Unlock()
	}

	// Otherwise fallback to fixed compact progress bars
	if !stdoutTTY {
		widthDelta = -pb.DefaultWidth
		renderProgressBars = func(goBack bool) {
			barText, _ := renderMultipleBars(stdoutTTY, goBack, maxLeft, termWidth, widthDelta, pbs, globalFlags)
			progressBarsLastRenderLock.Lock()
			progressBarsLastRender = []byte(barText)
			progressBarsLastRenderLock.Unlock()
		}
	}

	// TODO: make configurable?
	updateFreq := 1 * time.Second
	if stdoutTTY {
		updateFreq = 100 * time.Millisecond
		outMutex.Lock()
		stdout.PersistentText = printProgressBars
		stderr.PersistentText = printProgressBars
		outMutex.Unlock()
		defer func() {
			outMutex.Lock()
			stdout.PersistentText = nil
			stderr.PersistentText = nil
			outMutex.Unlock()
		}()
	}

	var (
		fd     = int(os.Stdout.Fd())
		ticker = time.NewTicker(updateFreq)
	)

	var winch chan os.Signal
	if sig := getWinchSignal(); sig != nil {
		winch = make(chan os.Signal, 1)
		signal.Notify(winch, sig)
	}

	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			renderProgressBars(false)
			outMutex.Lock()
			printProgressBars()
			outMutex.Unlock()
			return
		case <-winch:
			if stdoutTTY && !errTermGetSize {
				// More responsive progress bar resizing on platforms with SIGWINCH (*nix)
				tw, _, err := term.GetSize(fd)
				if tw > 0 && err == nil {
					termWidth = tw
				}
			}
		case <-ticker.C:
			// Default ticker-based progress bar resizing
			if stdoutTTY && !errTermGetSize && winch == nil {
				tw, _, err := term.GetSize(fd)
				if tw > 0 && err == nil {
					termWidth = tw
				}
			}
		}
		renderProgressBars(true)
		outMutex.Lock()
		printProgressBars()
		outMutex.Unlock()
	}
}

func yamlPrint(w io.Writer, v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}
	_, err = fmt.Fprint(w, string(data))
	if err != nil {
		return fmt.Errorf("could flush the data to the output: %w", err)
	}
	return nil
}
