package runtime

import (
	"io"
	"os"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/runtime/profiler"
)

// FakeExecutor is an instance of a ProcessExecutor that keeps the history
// of commands for inspection and returns the predefined results.
// Even when it allows multiple invocations to Exec, it only allows
// setting one err and output which are returned on each call. If different
// results are needed for each invocation, [CallbackExecutor] may a
// better alternative
type FakeExecutor struct {
	invocations int
	commands    []string
	err         error
	output      []byte
}

// NewFakeExecutor creates a new instance of a ProcessExecutor
func NewFakeExecutor(output []byte, err error) *FakeExecutor {
	return &FakeExecutor{
		err:    err,
		output: output,
	}
}

func (p *FakeExecutor) updateHistory(cmd string, args ...string) {
	cmdLine := cmd + " " + strings.Join(args, " ")
	p.commands = append(p.commands, cmdLine)
	p.invocations++
}

// Exec mocks the executing of the process according to
func (p *FakeExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	p.updateHistory(cmd, args...)
	return p.output, p.err
}

// Invoked indicates if the Exec command was invoked at least once
func (p *FakeExecutor) Invoked() bool {
	return p.invocations > 0
}

// Cmd returns the value of the last command passed to the last invocation
func (p *FakeExecutor) Cmd() string {
	if p.invocations == 0 {
		return ""
	}
	return p.commands[p.invocations-1]
}

// CmdHistory returns the history of commands executed. If Invocations is 0, returns
// an empty array
func (p *FakeExecutor) CmdHistory() []string {
	return p.commands
}

// Invocations returns the number of invocations to the Exec function
func (p *FakeExecutor) Invocations() int {
	return p.invocations
}

// Reset clears the history of invocations to the FakeProcessExecutor
func (p *FakeExecutor) Reset() {
	p.invocations = 0
	p.commands = []string{}
}

// ExecCallback defines a function that can receive the forward of an Exec invocation
// The function must return the output of the invocation and the execution error, if any
type ExecCallback func(cmd string, args ...string) ([]byte, error)

// CallbackExecutor is fake process Executor that forwards the invocations
// to a function that can dynamically return error and output.
type CallbackExecutor struct {
	FakeExecutor
	callback ExecCallback
}

// Exec forwards invocation to the callback
func (c *CallbackExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	// update command history but ignore outputs
	c.FakeExecutor.updateHistory(cmd, args...)
	// return outputs from callback
	return c.callback(cmd, args...)
}

// NewCallbackExecutor returns an instance of a CallbackExecutor
func NewCallbackExecutor(callback ExecCallback) *CallbackExecutor {
	return &CallbackExecutor{
		callback: callback,
	}
}

// FakeProfiler is a noop profiler for testing
type FakeProfiler struct {
	started bool
	stopped bool
}

// NewFakeProfiler creates a new FakeProfiler
func NewFakeProfiler() *FakeProfiler {
	return &FakeProfiler{}
}

// Start updates the FakeProfiler to registers it was started
func (p *FakeProfiler) Start(profiler.Config) (io.Closer, error) {
	p.started = true
	return p, nil
}

// Close updates the FakeProfiler to registers it was stopped operation
func (p *FakeProfiler) Close() error {
	p.stopped = true
	return nil
}

// FakeLock implements a Lock for testing
type FakeLock struct {
	locked   bool
	unlocked bool
	owner    int
}

// NewFakeLock returns a default FakeProcess for testing
func NewFakeLock() *FakeLock {
	return &FakeLock{}
}

// Acquire implements Acquire method from Lock interface
func (p *FakeLock) Acquire() (bool, error) {
	p.locked = true
	p.owner = os.Getpid()
	return true, nil
}

// Release implements Release method from Lock interface
func (p *FakeLock) Release() error {
	p.unlocked = true
	return nil
}

// Owner implements Owner method from Lock interface
func (p *FakeLock) Owner() int {
	if !p.locked {
		return -1
	}

	return p.owner
}

// FakeRuntime holds the state of a fake runtime for testing
type FakeRuntime struct {
	FakeArgs     []string
	FakeVars     map[string]string
	FakeExecutor *FakeExecutor
	FakeProfiler *FakeProfiler
	FakeLock     *FakeLock
	FakeSignal   *FakeSignal
}

// FakeSignal implements a fake signal handling for testing
type FakeSignal struct {
	channel chan os.Signal
}

// NewFakeSignal returns a FakeSignal
func NewFakeSignal() *FakeSignal {
	return &FakeSignal{
		channel: make(chan os.Signal),
	}
}

// Notify implements Signal's interface Notify method
func (f *FakeSignal) Notify(_ ...os.Signal) <-chan os.Signal {
	return f.channel
}

// Reset implements Signal's interface Reset method. It is noop.
func (f *FakeSignal) Reset(_ ...os.Signal) {
	// noop
}

// Send sends the given signal to the signal notification channel if the signal was
// previously specified in a call to Notify
func (f *FakeSignal) Send(signal os.Signal) {
	f.channel <- signal
}

// NewFakeRuntime creates a default FakeRuntime
func NewFakeRuntime(args []string, vars map[string]string) *FakeRuntime {
	return &FakeRuntime{
		FakeArgs:     args,
		FakeVars:     vars,
		FakeProfiler: NewFakeProfiler(),
		FakeExecutor: NewFakeExecutor(nil, nil),
		FakeLock:     NewFakeLock(),
		FakeSignal:   NewFakeSignal(),
	}
}

// Profiler implements Profiler method from Runtime interface
func (f *FakeRuntime) Profiler() profiler.Profiler {
	return f.FakeProfiler
}

// Executor implements Executor method from Runtime interface
func (f *FakeRuntime) Executor() Executor {
	return f.FakeExecutor
}

// Lock implements Lock method from Runtime interface
func (f *FakeRuntime) Lock() Lock {
	return f.FakeLock
}

// Vars implements Vars method from Runtime interface
func (f *FakeRuntime) Vars() map[string]string {
	return f.FakeVars
}

// Args implements Args method from Runtime interface
func (f *FakeRuntime) Args() []string {
	return f.FakeArgs
}

// Signal implements Signal method from Runtime interface
func (f *FakeRuntime) Signal() Signals {
	return f.FakeSignal
}
