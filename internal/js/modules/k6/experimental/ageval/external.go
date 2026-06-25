package ageval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

const (
	defaultExternalTimeoutSec = 300
	// inputToken is replaced in args with the run input; if absent, the input is
	// written to the command's stdin instead.
	inputToken = "{{input}}"
)

// CliAgent runs a real agent CLI as a subprocess, captures its output, and
// turns it into an AgentTestCase — so the agent runs as part of a single `k6 run`
// (no separate capture step). Output is parsed by a built-in `format` (currently
// "claude-code") or a custom `parse(stdout)` JS callback.
type CliAgent struct {
	mi *ModuleInstance
	rt *sobek.Runtime

	name           string
	command        string
	args           []string
	env            map[string]string
	cwd            string
	format         string
	parse          sobek.Callable
	model          string
	stepReportTool string
	timeoutSec     int
}

// newCliAgent is the `new CliAgent({...})` constructor.
//
//	{ command, args?, env?, cwd?, format? ("claude-code"), parse?(stdout)->trajectory,
//	  model?, name?, stepReportTool?, timeoutSeconds? }
func (mi *ModuleInstance) newCliAgent(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	if len(call.Arguments) < 1 || common.IsNullish(call.Argument(0)) {
		common.Throw(rt, errors.New("CliAgent constructor requires a config object"))
	}
	cfg := call.Argument(0).ToObject(rt)

	command := getString(cfg, "command", "")
	if command == "" {
		common.Throw(rt, errors.New("CliAgent config requires a non-empty `command`"))
	}

	a := &CliAgent{
		mi:             mi,
		rt:             rt,
		name:           getString(cfg, "name", "external-agent"),
		command:        command,
		args:           toStringSlice(rt, cfg.Get("args")),
		env:            toStringMap(rt, cfg.Get("env")),
		cwd:            getString(cfg, "cwd", ""),
		format:         getString(cfg, "format", ""),
		model:          getString(cfg, "model", ""),
		stepReportTool: getString(cfg, "stepReportTool", defaultStepReportTool),
		timeoutSec:     getInt(cfg, "timeoutSeconds", defaultExternalTimeoutSec),
	}
	if p := cfg.Get("parse"); p != nil && !common.IsNullish(p) {
		if fn, ok := sobek.AssertFunction(p); ok {
			a.parse = fn
		} else {
			common.Throw(rt, errors.New("CliAgent `parse` must be a function"))
		}
	}
	return rt.ToValue(a).ToObject(rt)
}

// Run executes the agent command with the given input, parses its output, and
// returns an AgentTestCase. Blocking (like http.request) — runs on the VU goroutine.
//
//	run({ input, expectedTools?: [{ name, input? }], tags? })
//
// expectedTools is attached to the returned AgentTestCase so expectSequence() can
// grade against it with no argument.
func (a *CliAgent) Run(opts sobek.Value) sobek.Value {
	state := a.mi.vu.State()
	if state == nil {
		common.Throw(a.rt, errInitContext)
	}
	if opts == nil || common.IsNullish(opts) {
		common.Throw(a.rt, errors.New("run() requires an options object with an `input`"))
	}
	o := opts.ToObject(a.rt)
	input := getString(o, "input", "")
	if input == "" {
		common.Throw(a.rt, errors.New("run() requires a non-empty `input`"))
	}

	stdout, elapsed, err := a.execCommand(input)
	if err != nil {
		common.Throw(a.rt, err)
	}

	tr := a.parseOutput(stdout)
	tags := a.mi.realRunTags(state, a.name, a.model, o.Get("tags"))

	result := a.mi.newRealAgentTestCase(a.rt, realRunData{
		tags:           tags,
		stepReportTool: a.stepReportTool,
		input:          input,
		output:         tr.output,
		model:          a.model,
		toolCalls:      tr.toolCalls,
		inTok:          tr.inTok,
		outTok:         tr.outTok,
		durationMs:     float64(elapsed) / float64(time.Millisecond),
		steps:          len(tr.toolCalls),
	})
	result.ExpectedTools = parseToolCalls(a.rt, o.Get("expectedTools"))
	return a.rt.ToValue(result).ToObject(a.rt)
}

// execCommand runs the agent CLI and returns its stdout and wall-clock duration.
func (a *CliAgent) execCommand(input string) (string, time.Duration, error) {
	ctx := a.mi.vu.Context()
	if a.timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(a.timeoutSec)*time.Second)
		defer cancel()
	}

	args := make([]string, len(a.args))
	substituted := false
	for i, arg := range a.args {
		if strings.Contains(arg, inputToken) {
			args[i] = strings.ReplaceAll(arg, inputToken, input)
			substituted = true
		} else {
			args[i] = arg
		}
	}

	// The command and args come from the test author (the same trust level as the
	// k6 script itself), not from untrusted external input.
	cmd := exec.CommandContext(ctx, a.command, args...) //nolint:gosec // G204: command is operator-supplied
	if a.cwd != "" {
		cmd.Dir = a.cwd
	}
	// Leaving cmd.Env nil makes the child inherit the parent environment (so the
	// agent CLI sees its own credentials, PATH, etc.). Only build an explicit
	// env when extra vars are supplied. cmd.Environ() avoids importing os.
	if len(a.env) > 0 {
		env := cmd.Environ()
		for k, v := range a.env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}
	if !substituted {
		cmd.Stdin = strings.NewReader(input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)
	if runErr != nil {
		return "", elapsed, fmt.Errorf("external agent %q failed: %w: %s",
			a.command, runErr, tail(stderr.String(), 500))
	}
	return stdout.String(), elapsed, nil
}

// parseOutput turns the command's stdout into a trajectory using the custom
// parse callback, a built-in format adapter, or (default) the raw text as the
// output.
func (a *CliAgent) parseOutput(stdout string) trajectory {
	if a.parse != nil {
		res, err := a.parse(sobek.Undefined(), a.rt.ToValue(stdout))
		if err != nil {
			common.Throw(a.rt, fmt.Errorf("CliAgent parse callback threw: %w", err))
		}
		return trajectoryFromJS(a.rt, res)
	}
	if a.format != "" {
		adapter, ok := lookupAdapter(a.format)
		if !ok {
			common.Throw(a.rt, fmt.Errorf("unknown format %q (supported: %s)",
				a.format, strings.Join(adapterNames(), ", ")))
		}
		return adapter(stdout)
	}
	return trajectory{output: strings.TrimSpace(stdout)}
}

// trajectoryFromJS converts the object returned by a `parse` callback or passed
// to new AgentTestCase({...}) into a trajectory.
func trajectoryFromJS(rt *sobek.Runtime, v sobek.Value) trajectory {
	if v == nil || common.IsNullish(v) {
		return trajectory{}
	}
	obj := v.ToObject(rt)
	t := trajectory{output: getString(obj, "output", ""), toolCalls: parseToolCalls(rt, obj.Get("toolCalls"))}
	if uv := obj.Get("usage"); uv != nil && !common.IsNullish(uv) {
		uo := uv.ToObject(rt)
		t.inTok = int64(getInt(uo, "inputTokens", 0))
		t.outTok = int64(getInt(uo, "outputTokens", 0))
	}
	return t
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// toStringSlice converts a JS array to []string.
func toStringSlice(rt *sobek.Runtime, v sobek.Value) []string {
	if v == nil || common.IsNullish(v) {
		return nil
	}
	arr := v.ToObject(rt)
	lengthVal := arr.Get("length")
	if lengthVal == nil {
		return nil
	}
	n := int(lengthVal.ToInteger())
	out := make([]string, 0, n)
	for i := range n {
		item := arr.Get(strconv.Itoa(i))
		if item == nil || sobek.IsUndefined(item) {
			continue
		}
		out = append(out, item.String())
	}
	return out
}

// toStringMap converts a JS object to map[string]string.
func toStringMap(rt *sobek.Runtime, v sobek.Value) map[string]string {
	if v == nil || common.IsNullish(v) {
		return nil
	}
	obj := v.ToObject(rt)
	out := map[string]string{}
	for _, k := range obj.Keys() {
		out[k] = obj.Get(k).String()
	}
	return out
}
