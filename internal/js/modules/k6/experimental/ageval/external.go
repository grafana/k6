package ageval

import (
	"bytes"
	"context"
	"encoding/json"
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

// ExternalAgent runs a real agent CLI as a subprocess, captures its output, and
// turns it into a RunResult — so the agent runs as part of a single `k6 run`
// (no separate capture step). Output is parsed by a built-in `format` (currently
// "claude-code") or a custom `parse(stdout)` JS callback.
type ExternalAgent struct {
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

// newExternalAgent is the `new ExternalAgent({...})` constructor.
//
//	{ command, args?, env?, cwd?, format? ("claude-code"), parse?(stdout)->trajectory,
//	  model?, name?, stepReportTool?, timeoutSeconds? }
func (mi *ModuleInstance) newExternalAgent(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	if len(call.Arguments) < 1 || common.IsNullish(call.Argument(0)) {
		common.Throw(rt, errors.New("ExternalAgent constructor requires a config object"))
	}
	cfg := call.Argument(0).ToObject(rt)

	command := getString(cfg, "command", "")
	if command == "" {
		common.Throw(rt, errors.New("ExternalAgent config requires a non-empty `command`"))
	}

	a := &ExternalAgent{
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
			common.Throw(rt, errors.New("ExternalAgent `parse` must be a function"))
		}
	}
	return rt.ToValue(a).ToObject(rt)
}

// Run executes the agent command with the given input, parses its output, and
// returns a RunResult. Blocking (like http.request) — runs on the VU goroutine.
func (a *ExternalAgent) Run(opts sobek.Value) sobek.Value {
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

	output, toolCalls, inTok, outTok := a.parseOutput(stdout)
	tags := a.mi.realRunTags(state, a.name, a.model, o.Get("tags"))

	result := a.mi.newRealRunResult(a.rt, realRunData{
		tags:           tags,
		stepReportTool: a.stepReportTool,
		input:          input,
		output:         output,
		model:          a.model,
		toolCalls:      toolCalls,
		inTok:          inTok,
		outTok:         outTok,
		durationMs:     float64(elapsed) / float64(time.Millisecond),
		steps:          len(toolCalls),
	})
	return a.rt.ToValue(result).ToObject(a.rt)
}

// execCommand runs the agent CLI and returns its stdout and wall-clock duration.
func (a *ExternalAgent) execCommand(input string) (string, time.Duration, error) {
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
// parse callback, the built-in format, or (default) the raw text as the output.
func (a *ExternalAgent) parseOutput(stdout string) (string, []ToolCall, int64, int64) {
	if a.parse != nil {
		res, err := a.parse(sobek.Undefined(), a.rt.ToValue(stdout))
		if err != nil {
			common.Throw(a.rt, fmt.Errorf("ExternalAgent parse callback threw: %w", err))
		}
		if res == nil || common.IsNullish(res) {
			return "", nil, 0, 0
		}
		obj := res.ToObject(a.rt)
		var inTok, outTok int64
		if uv := obj.Get("usage"); uv != nil && !common.IsNullish(uv) {
			uo := uv.ToObject(a.rt)
			inTok = int64(getInt(uo, "inputTokens", 0))
			outTok = int64(getInt(uo, "outputTokens", 0))
		}
		return getString(obj, "output", ""), parseToolCalls(a.rt, obj.Get("toolCalls")), inTok, outTok
	}
	if a.format == "claude-code" {
		return parseClaudeCodeTranscript(stdout)
	}
	return strings.TrimSpace(stdout), nil, 0, 0
}

// --- Claude Code stream-json transcript parsing ---

type ccBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type ccEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []ccBlock `json:"content"`
	} `json:"message"`
	Result *string `json:"result"`
	Usage  *struct {
		InputTokens   int64 `json:"input_tokens"`
		OutputTokens  int64 `json:"output_tokens"`
		CacheRead     int64 `json:"cache_read_input_tokens"`
		CacheCreation int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// parseClaudeCodeTranscript parses a Claude Code `--output-format stream-json`
// transcript (JSONL) into a trajectory, normalizing mcp__<server>__<tool> names.
func parseClaudeCodeTranscript(jsonl string) (string, []ToolCall, int64, int64) {
	var (
		output        string
		toolCalls     = []ToolCall{}
		byID          = map[string]int{}
		inTok, outTok int64
	)
	for line := range strings.SplitSeq(jsonl, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e ccEvent
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		switch e.Type {
		case "assistant":
			ccHandleAssistant(e.Message.Content, &toolCalls, byID, &output)
		case "user":
			ccHandleToolResults(e.Message.Content, toolCalls, byID)
		case "result":
			if e.Result != nil {
				output = *e.Result
			}
			if e.Usage != nil {
				inTok = e.Usage.InputTokens + e.Usage.CacheRead + e.Usage.CacheCreation
				outTok = e.Usage.OutputTokens
			}
		}
	}
	return output, toolCalls, inTok, outTok
}

// ccHandleAssistant records tool calls and the latest text from an assistant event.
func ccHandleAssistant(content []ccBlock, toolCalls *[]ToolCall, byID map[string]int, output *string) {
	for _, b := range content {
		switch b.Type {
		case blockToolUse:
			input := map[string]any{}
			if len(b.Input) > 0 {
				_ = json.Unmarshal(b.Input, &input)
			}
			*toolCalls = append(*toolCalls, ToolCall{Name: normalizeMCPToolName(b.Name), Input: input})
			byID[b.ID] = len(*toolCalls) - 1
		case blockText:
			if b.Text != "" {
				*output = b.Text
			}
		}
	}
}

// ccHandleToolResults attaches tool_result outputs to their matching tool calls.
func ccHandleToolResults(content []ccBlock, toolCalls []ToolCall, byID map[string]int) {
	for _, b := range content {
		if b.Type != blockToolResult {
			continue
		}
		if idx, ok := byID[b.ToolUseID]; ok {
			toolCalls[idx].Output = stringifyJSONContent(b.Content)
		}
	}
}

// normalizeMCPToolName strips the "mcp__<server>__" prefix Claude Code adds to
// MCP tool calls, so assertions use the server's own tool names.
func normalizeMCPToolName(name string) string {
	parts := strings.Split(name, "__")
	if len(parts) >= 3 && parts[0] == "mcp" {
		return strings.Join(parts[2:], "__")
	}
	return name
}

// stringifyJSONContent renders a tool_result `content` (string or array of
// blocks) as a plain string.
func stringifyJSONContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &blocks) == nil {
		parts := make([]string, 0, len(blocks))
		for _, b := range blocks {
			parts = append(parts, b.Text)
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
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
