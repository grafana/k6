package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	"gopkg.in/yaml.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/metrics/engine"
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

func printBanner(gs *state.GlobalState) {
	if gs.Flags.Quiet {
		return // do not print banner when --quiet is enabled
	}

	banner := getBanner(gs.Flags.NoColor || !gs.Stdout.IsTTY)
	_, err := fmt.Fprintf(gs.Stdout, "\n%s\n\n", banner)
	if err != nil {
		gs.Logger.Warnf("could not print k6 banner message to stdout: %s", err.Error())
	}
}

func printBar(gs *state.GlobalState, bar *pb.ProgressBar) {
	if gs.Flags.Quiet {
		return
	}
	end := "\n"
	// TODO: refactor widthDelta away? make the progressbar rendering a bit more
	// stateless... basically first render the left and right parts, so we know
	// how long the longest line is, and how much space we have for the progress
	widthDelta := -defaultTermWidth
	if gs.Stdout.IsTTY {
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
	printToStdout(gs, rendered.String()+end)
}

func modifyAndPrintBar(gs *state.GlobalState, bar *pb.ProgressBar, options ...pb.ProgressBarOption) {
	bar.Modify(options...)
	printBar(gs, bar)
}

// Print execution description for both cloud and local execution.
// TODO: Clean this up as part of #1499 or #1427
func printExecutionDescription(
	gs *state.GlobalState, execution, filename, outputOverride string, conf Config,
	et *lib.ExecutionTuple, execPlan []lib.ExecutionStep, outputs []output.Output,
) {
	noColor := gs.Flags.NoColor || !gs.Stdout.IsTTY
	valueColor := getColor(noColor, color.FgCyan)

	buf := &strings.Builder{}
	fmt.Fprintf(buf, "  execution: %s\n", valueColor.Sprint(execution))
	fmt.Fprintf(buf, "     script: %s\n", valueColor.Sprint(filename))

	var outputDescriptions []string
	switch {
	case outputOverride != "":
		outputDescriptions = []string{outputOverride}
	default:
		for _, out := range outputs {
			desc := out.Description()
			if desc == engine.IngesterDescription {
				if len(outputs) != 1 {
					continue
				}
				desc = "-"
			}
			outputDescriptions = append(outputDescriptions, desc)
		}
	}

	fmt.Fprintf(buf, "     output: %s\n", valueColor.Sprint(strings.Join(outputDescriptions, ", ")))
	if gs.Flags.ProfilingEnabled && gs.Flags.Address != "" {
		fmt.Fprintf(buf, "  profiling: %s\n", valueColor.Sprintf("http://%s/debug/pprof/", gs.Flags.Address))
	}

	fmt.Fprintf(buf, "\n")

	maxDuration, _ := lib.GetEndOffset(execPlan)
	executorConfigs := conf.Scenarios.GetSortedConfigs()

	scenarioDesc := "1 scenario"
	if len(executorConfigs) > 1 {
		scenarioDesc = fmt.Sprintf("%d scenarios", len(executorConfigs))
	}

	fmt.Fprintf(buf, "  scenarios: %s\n", valueColor.Sprintf(
		"(%.2f%%) %s, %d max VUs, %s max duration (incl. graceful stop):",
		conf.ExecutionSegment.FloatLength()*100, scenarioDesc,
		lib.GetMaxPossibleVUs(execPlan), maxDuration.Round(100*time.Millisecond)),
	)
	for _, ec := range executorConfigs {
		fmt.Fprintf(buf, "           * %s: %s\n",
			ec.GetName(), ec.GetDescription(et))
	}
	fmt.Fprintf(buf, "\n")

	if gs.Flags.Quiet {
		gs.Logger.Debug(buf.String())
	} else {
		printToStdout(gs, buf.String())
	}
}

//nolint:funlen
func renderMultipleBars(
	nocolor, isTTY, goBack bool, maxLeft, termWidth, widthDelta int, pbs []*pb.ProgressBar,
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
		if !nocolor {
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
//
//nolint:funlen,gocognit
func showProgress(ctx context.Context, gs *state.GlobalState, pbs []*pb.ProgressBar, logger logrus.FieldLogger) {
	if gs.Flags.Quiet {
		return
	}

	var errTermGetSize bool
	termWidth := defaultTermWidth
	if gs.Stdout.IsTTY {
		tw, _, err := term.GetSize(int(gs.Stdout.RawOut.Fd()))
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
		_, _ = gs.Stdout.Writer.Write(progressBarsLastRender)
		progressBarsLastRenderLock.Unlock()
	}

	var widthDelta int
	// Default to responsive progress bars when in an interactive terminal
	renderProgressBars := func(goBack bool) {
		barText, longestLine := renderMultipleBars(
			gs.Flags.NoColor, gs.Stdout.IsTTY, goBack, maxLeft, termWidth, widthDelta, pbs,
		)
		widthDelta = termWidth - longestLine - termPadding
		progressBarsLastRenderLock.Lock()
		progressBarsLastRender = []byte(barText)
		progressBarsLastRenderLock.Unlock()
	}

	// Otherwise fallback to fixed compact progress bars
	if !gs.Stdout.IsTTY {
		widthDelta = -pb.DefaultWidth
		renderProgressBars = func(goBack bool) {
			barText, _ := renderMultipleBars(gs.Flags.NoColor, gs.Stdout.IsTTY, goBack, maxLeft, termWidth, widthDelta, pbs)
			progressBarsLastRenderLock.Lock()
			progressBarsLastRender = []byte(barText)
			progressBarsLastRenderLock.Unlock()
		}
	}

	// TODO: make configurable?
	updateFreq := 1 * time.Second
	var stdoutFD int
	if gs.Stdout.IsTTY {
		stdoutFD = int(gs.Stdout.RawOut.Fd())
		updateFreq = 100 * time.Millisecond
		gs.OutMutex.Lock()
		gs.Stdout.PersistentText = printProgressBars
		gs.Stderr.PersistentText = printProgressBars
		gs.OutMutex.Unlock()
		defer func() {
			gs.OutMutex.Lock()
			gs.Stdout.PersistentText = nil
			gs.Stderr.PersistentText = nil
			gs.OutMutex.Unlock()
		}()
	}

	var winch chan os.Signal
	if sig := getWinchSignal(); sig != nil {
		winch = make(chan os.Signal, 10)
		gs.SignalNotify(winch, sig)
		defer gs.SignalStop(winch)
	}

	ticker := time.NewTicker(updateFreq)
	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			renderProgressBars(false)
			gs.OutMutex.Lock()
			printProgressBars()
			gs.OutMutex.Unlock()
			return
		case <-winch:
			if gs.Stdout.IsTTY && !errTermGetSize {
				// More responsive progress bar resizing on platforms with SIGWINCH (*nix)
				tw, _, err := term.GetSize(stdoutFD)
				if tw > 0 && err == nil {
					termWidth = tw
				}
			}
		case <-ticker.C:
			// Default ticker-based progress bar resizing
			if gs.Stdout.IsTTY && !errTermGetSize && winch == nil {
				tw, _, err := term.GetSize(stdoutFD)
				if tw > 0 && err == nil {
					termWidth = tw
				}
			}
		}
		renderProgressBars(true)
		gs.OutMutex.Lock()
		printProgressBars()
		gs.OutMutex.Unlock()
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
