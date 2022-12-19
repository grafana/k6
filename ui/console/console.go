package console

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"

	"gopkg.in/yaml.v3"
)

// Console enables synced writing to stdout and stderr ...
type Console struct {
	IsTTY          bool
	outMx          *sync.Mutex
	Stdout, Stderr OSFileW
	Stdin          OSFileR
	rawStdout      io.Writer
	stdout, stderr *consoleWriter
	theme          *theme
	signalNotify   func(chan<- os.Signal, ...os.Signal)
	signalStop     func(chan<- os.Signal)
	logger         *logrus.Logger
}

// New returns the pointer to a new Console value.
func New(
	stdout, stderr OSFileW, stdin OSFileR,
	colorize bool, termType string,
	signalNotify func(chan<- os.Signal, ...os.Signal),
	signalStop func(chan<- os.Signal),
) *Console {
	outMx := &sync.Mutex{}
	outCW := newConsoleWriter(stdout, outMx, termType)
	errCW := newConsoleWriter(stderr, outMx, termType)
	isTTY := outCW.isTTY && errCW.isTTY

	// Default logger without any formatting
	logger := &logrus.Logger{
		Out:       stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
	}

	var th *theme
	// Only enable themes and a fancy logger if we're in a TTY
	if isTTY && colorize {
		th = &theme{foreground: newColor(color.FgCyan)}

		logger = &logrus.Logger{
			Out: stderr,
			Formatter: &logrus.TextFormatter{
				ForceColors:   true,
				DisableColors: false,
			},
			Hooks: make(logrus.LevelHooks),
			Level: logrus.InfoLevel,
		}
	}

	return &Console{
		IsTTY:        isTTY,
		outMx:        outMx,
		Stdout:       stdout,
		Stderr:       stderr,
		Stdin:        stdin,
		rawStdout:    stdout,
		stdout:       outCW,
		stderr:       errCW,
		theme:        th,
		signalNotify: signalNotify,
		signalStop:   signalStop,
		logger:       logger,
	}
}

// ApplyTheme adds ANSI color escape sequences to s if themes are enabled;
// otherwise it returns s unchanged.
func (c *Console) ApplyTheme(s string) string {
	if c.colorized() {
		return c.theme.foreground.Sprint(s)
	}

	return s
}

// Banner returns the k6.io ASCII art banner, optionally with ANSI color escape
// sequences if themes are enabled.
func (c *Console) Banner() string {
	banner := strings.Join([]string{
		`          /\      |‾‾| /‾‾/   /‾‾/   `,
		`     /\  /  \     |  |/  /   /  /    `,
		`    /  \/    \    |     (   /   ‾‾\  `,
		`   /          \   |  |\  \ |  (‾)  | `,
		`  / __________ \  |__| \__\ \_____/ .io`,
	}, "\n")

	return c.ApplyTheme(banner)
}

// GetLogger returns the preconfigured plain-text logger. It will be configured
// to output colors if themes are enabled.
func (c *Console) GetLogger() *logrus.Logger {
	return c.logger
}

// SetLogger overrides the preconfigured logger.
func (c *Console) SetLogger(l *logrus.Logger) {
	c.logger = l
}

// Print writes s to stdout.
func (c *Console) Print(s string) {
	if _, err := fmt.Fprint(c.Stdout, s); err != nil {
		c.logger.Errorf("could not print '%s' to stdout: %s", s, err.Error())
	}
}

// Printf writes s to stdout, formatted with optional arguments.
func (c *Console) Printf(s string, a ...interface{}) {
	if _, err := fmt.Fprintf(c.Stdout, s, a...); err != nil {
		c.logger.Errorf("could not print '%s' to stdout: %s", s, err.Error())
	}
}

// PrintYAML marshals v to YAML, and writes the result to stdout. It returns an
// error if marshalling fails.
func (c *Console) PrintYAML(v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("could not marshal YAML: %w", err)
	}
	c.Print(string(data))
	return nil
}

// TermWidth returns the terminal window width in characters. If the window size
// lookup fails, or if we're not running in a TTY (interactive terminal), the
// default value of 80 will be returned. err will be non-nil if the lookup fails.
func (c *Console) TermWidth() (int, error) {
	if !c.IsTTY {
		return defaultTermWidth, nil
	}

	width, _, err := term.GetSize(int(c.Stdout.Fd()))
	if !(width > 0) || err != nil {
		return defaultTermWidth, err
	}

	return width, nil
}

func (c *Console) colorized() bool {
	return c.theme != nil
}

func (c *Console) setPersistentText(pt func()) {
	c.outMx.Lock()
	defer c.outMx.Unlock()

	c.stdout.persistentText = pt
	c.stderr.persistentText = pt
}

// OSFile is a subset of the functionality implemented by os.File.
type OSFile interface {
	Fd() uintptr
}

// OSFileW is the writer variant of OSFile, typically representing os.Stdout and
// os.Stderr.
type OSFileW interface {
	io.Writer
	OSFile
}

// OSFileR is the reader variant of OSFile, typically representing os.Stdin.
type OSFileR interface {
	io.Reader
	OSFile
}

// theme is a collection of colors supported by the console output.
type theme struct {
	foreground *color.Color
}

// A writer that syncs writes with a mutex and, if the output is a TTY, clears
// before newlines.
type consoleWriter struct {
	OSFileW
	isTTY bool
	mutex *sync.Mutex

	// Used for flicker-free persistent objects like the progressbars
	persistentText func()
}

func newConsoleWriter(out OSFileW, mx *sync.Mutex, termType string) *consoleWriter {
	isTTY := termType != "dumb" && (isatty.IsTerminal(out.Fd()) || isatty.IsCygwinTerminal(out.Fd()))
	return &consoleWriter{out, isTTY, mx, nil}
}

func (w *consoleWriter) Write(p []byte) (n int, err error) {
	origLen := len(p)
	if w.isTTY {
		// Add a TTY code to erase till the end of line with each new line
		// TODO: check how cross-platform this is...
		p = bytes.ReplaceAll(p, []byte{'\n'}, []byte{'\x1b', '[', '0', 'K', '\n'})
	}

	w.mutex.Lock()
	n, err = w.OSFileW.Write(p)
	if w.persistentText != nil {
		w.persistentText()
	}
	w.mutex.Unlock()

	if err != nil && n < origLen {
		return n, err
	}
	return origLen, err
}

// newColor returns the requested color with the given attributes.
func newColor(attributes ...color.Attribute) *color.Color {
	c := color.New(attributes...)
	c.EnableColor()
	return c
}
