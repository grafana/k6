package ageval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	sigilv1 "go.k6.io/k6/v2/internal/js/modules/k6/experimental/ageval/sigilv1"
	"go.k6.io/k6/v2/js/common"
)

const (
	defaultSigilTimeoutSec = 300
	defaultSigilAgentName  = "sigil-agent"
	sigilMaxRecvBytes      = 16 << 20

	// After the subprocess exits, wait until no new generation has arrived for
	// sigilFlushQuiet (the agent's final SDK flush is in flight), capped at
	// sigilFlushMaxWait.
	sigilFlushQuiet   = 200 * time.Millisecond
	sigilFlushMaxWait = 3 * time.Second
)

// SigilAgent runs a real, Sigil-instrumented agent CLI as a subprocess and
// builds its AgentTestCase from the telemetry the agent streams back over a
// gRPC endpoint ageval hosts locally — rather than parsing stdout. ageval wires
// the agent to that endpoint via SIGIL_* env vars and correlates the stream to
// this run with a unique test_run_id tag, so it works under k6's VUs/iterations.
//
// The instrumented agent MUST flush its Sigil client before exiting (e.g.
// client.Shutdown()), otherwise its final generations may not arrive in time.
type SigilAgent struct {
	mi *ModuleInstance
	rt *sobek.Runtime

	name           string
	command        string
	args           []string
	env            map[string]string
	cwd            string
	model          string
	stepReportTool string
	timeoutSec     int
}

// newSigilAgent is the `new SigilAgent({...})` constructor.
//
//	{ command, args?, env?, cwd?, model?, name?, stepReportTool?, timeoutSeconds? }
func (mi *ModuleInstance) newSigilAgent(call sobek.ConstructorCall) *sobek.Object {
	rt := mi.vu.Runtime()
	if len(call.Arguments) < 1 || common.IsNullish(call.Argument(0)) {
		common.Throw(rt, errors.New("SigilAgent constructor requires a config object"))
	}
	cfg := call.Argument(0).ToObject(rt)

	command := getString(cfg, "command", "")
	if command == "" {
		common.Throw(rt, errors.New("SigilAgent config requires a non-empty `command`"))
	}

	a := &SigilAgent{
		mi:             mi,
		rt:             rt,
		name:           getString(cfg, "name", defaultSigilAgentName),
		command:        command,
		args:           toStringSlice(rt, cfg.Get("args")),
		env:            toStringMap(rt, cfg.Get("env")),
		cwd:            getString(cfg, "cwd", ""),
		model:          getString(cfg, "model", ""),
		stepReportTool: getString(cfg, "stepReportTool", defaultStepReportTool),
		timeoutSec:     getInt(cfg, "timeoutSeconds", defaultSigilTimeoutSec),
	}

	// Start the shared ingest server eagerly so the endpoint is ready and any
	// bind error surfaces at construction rather than mid-run.
	mi.root.sigilServerInstance(rt)

	return rt.ToValue(a).ToObject(rt)
}

// Run executes the agent with the given input, collects the Sigil generations it
// streams during the run, and returns an AgentTestCase. Blocking (like
// http.request) — runs on the VU goroutine.
//
//	run({ input, expectedTools?: [{ name, input? }], tags? })
func (a *SigilAgent) Run(opts sobek.Value) sobek.Value {
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

	srv := a.mi.root.sigilServerInstance(a.rt)
	runID := srv.nextRunID()
	coll := srv.register(runID)
	defer srv.deregister(runID)

	stdout, elapsed, err := runSubprocess(a.mi.vu.Context(), procSpec{
		command:    a.command,
		args:       a.args,
		env:        a.sigilEnv(srv.addr, runID),
		cwd:        a.cwd,
		input:      input,
		timeoutSec: a.timeoutSec,
	})
	if err != nil {
		common.Throw(a.rt, err)
	}

	// Drain the generations this run streamed (waiting briefly for the agent's
	// final flush to settle), then map them into a trajectory.
	gens := coll.drain(sigilFlushQuiet, sigilFlushMaxWait)
	tr := sigilToTrajectory(gens)
	if tr.output == "" {
		// Fall back to stdout if the agent didn't emit a final assistant message.
		tr.output = strings.TrimSpace(stdout)
	}

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
		steps:          len(gens), // model round-trips, like the simulator's steps
	})
	result.ExpectedTools = parseToolCalls(a.rt, o.Get("expectedTools"))

	srv.recordSummary(sigilRunSummary{
		runID:        runID,
		agent:        a.name,
		generations:  len(gens),
		toolCalls:    tr.toolCalls,
		inputTokens:  tr.inTok,
		outputTokens: tr.outTok,
		output:       tr.output,
	})

	return a.rt.ToValue(result).ToObject(a.rt)
}

// sigilEnv builds the extra env appended onto the inherited environment by
// runSubprocess: the test's own env first, then the SIGIL_* wiring last (so it
// wins on duplicate keys).
func (a *SigilAgent) sigilEnv(addr, runID string) []string {
	env := make([]string, 0, len(a.env)+6)
	for k, v := range a.env {
		env = append(env, k+"="+v)
	}
	env = append(env,
		"SIGIL_ENDPOINT="+addr,
		"SIGIL_PROTOCOL=grpc",
		"SIGIL_INSECURE=true",
		"SIGIL_TAGS=test_run_id="+runID,
		"SIGIL_CONTENT_CAPTURE_MODE=full",
	)
	if a.name != "" {
		env = append(env, "SIGIL_AGENT_NAME="+a.name)
	}
	return env
}

// --- subprocess execution (shared shape with CliAgent) ---

type procSpec struct {
	command    string
	args       []string
	env        []string
	cwd        string
	input      string
	timeoutSec int
}

// runSubprocess runs a command with the given input and returns stdout and the
// wall-clock duration. The input is substituted into any arg containing
// inputToken; if none does, it is written to stdin.
func runSubprocess(ctx context.Context, spec procSpec) (string, time.Duration, error) {
	if spec.timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(spec.timeoutSec)*time.Second)
		defer cancel()
	}

	args, substituted := substituteInput(spec.args, spec.input)

	// The command and args are operator-supplied (same trust level as the k6
	// script), not untrusted external input.
	cmd := exec.CommandContext(ctx, spec.command, args...) //nolint:gosec // G204: operator-supplied
	if spec.cwd != "" {
		cmd.Dir = spec.cwd
	}
	// Inherit the parent environment and append the extra vars. cmd.Environ()
	// avoids importing os (forbidden by the linter).
	if len(spec.env) > 0 {
		cmd.Env = append(cmd.Environ(), spec.env...)
	}
	if !substituted {
		cmd.Stdin = strings.NewReader(spec.input)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)
	if runErr != nil {
		return "", elapsed, fmt.Errorf("sigil agent %q failed: %w: %s",
			spec.command, runErr, tail(stderr.String(), 500))
	}
	return stdout.String(), elapsed, nil
}

func substituteInput(args []string, input string) ([]string, bool) {
	out := make([]string, len(args))
	substituted := false
	for i, arg := range args {
		if strings.Contains(arg, inputToken) {
			out[i] = strings.ReplaceAll(arg, inputToken, input)
			substituted = true
		} else {
			out[i] = arg
		}
	}
	return out, substituted
}

// --- Generation → trajectory mapping ---

// sigilToTrajectory reduces the Sigil generations of one run into the normalized
// trajectory. Tool calls are read from assistant output parts (indexed by id);
// tool results — which arrive as parts on later generations' inputs — are matched
// back by tool_call_id to fill outputs. Usage is summed; the final assistant text
// is the output.
func sigilToTrajectory(gens []*sigilv1.Generation) trajectory {
	t := trajectory{toolCalls: []ToolCall{}}
	byID := map[string]int{}
	for _, g := range sortedByStart(gens) {
		collectOutputs(g, &t, byID)
		applyToolResults(g, &t, byID)
		if u := g.GetUsage(); u != nil {
			t.inTok += u.GetInputTokens() + u.GetCacheReadInputTokens() + u.GetCacheWriteInputTokens()
			t.outTok += u.GetOutputTokens()
		}
	}
	return t
}

// sortedByStart returns a copy of gens ordered by started_at.
func sortedByStart(gens []*sigilv1.Generation) []*sigilv1.Generation {
	sorted := make([]*sigilv1.Generation, len(gens))
	copy(sorted, gens)
	sort.SliceStable(sorted, func(i, j int) bool {
		return genStart(sorted[i]).Before(genStart(sorted[j]))
	})
	return sorted
}

// collectOutputs records tool calls (indexed by id) and the latest assistant
// text from a generation's output messages.
func collectOutputs(g *sigilv1.Generation, t *trajectory, byID map[string]int) {
	for _, m := range g.GetOutput() {
		for _, p := range m.GetParts() {
			if tc := p.GetToolCall(); tc != nil {
				t.toolCalls = append(t.toolCalls, ToolCall{
					Name:  tc.GetName(),
					Input: rawToMap(json.RawMessage(tc.GetInputJson())),
				})
				if id := tc.GetId(); id != "" {
					byID[id] = len(t.toolCalls) - 1
				}
				continue
			}
			if txt := p.GetText(); txt != "" {
				t.output = txt
			}
		}
	}
}

// applyToolResults matches tool results (which arrive on a generation's input
// messages) back to the recorded tool calls by tool_call_id.
func applyToolResults(g *sigilv1.Generation, t *trajectory, byID map[string]int) {
	for _, m := range g.GetInput() {
		for _, p := range m.GetParts() {
			res := p.GetToolResult()
			if res == nil {
				continue
			}
			if idx, ok := byID[res.GetToolCallId()]; ok {
				t.toolCalls[idx].Output = toolResultString(res)
			}
		}
	}
}

func toolResultString(res *sigilv1.ToolResult) string {
	if c := res.GetContent(); c != "" {
		return c
	}
	if cj := res.GetContentJson(); len(cj) > 0 {
		return string(cj)
	}
	return ""
}

func genStart(g *sigilv1.Generation) time.Time {
	if ts := g.GetStartedAt(); ts != nil {
		return ts.AsTime()
	}
	return time.Time{}
}

// --- shared in-process Sigil ingest server ---

// sigilServerInstance lazily starts the process-wide Sigil ingest server shared
// by all VUs, returning it. It panics into JS on bind failure.
func (rm *RootModule) sigilServerInstance(rt *sobek.Runtime) *sigilServer {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if rm.server == nil {
		srv, err := startSigilServer()
		if err != nil {
			common.Throw(rt, err)
		}
		rm.server = srv
	}
	return rm.server
}

// peekSigilServer returns the shared server without starting one (nil if no
// SigilAgent has been constructed this test).
func (rm *RootModule) peekSigilServer() *sigilServer {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.server
}

// sigilSummary aggregates every run's Sigil data collected this test, logs a
// human-readable report, and returns the aggregate as a JS object. Call it once
// at the end of the test (e.g. from teardown()):
//
//	import { sigilSummary } from 'k6/experimental/ageval';
//	export function teardown() { sigilSummary(); }
func (mi *ModuleInstance) sigilSummary() sobek.Value {
	rt := mi.vu.Runtime()

	var runs []sigilRunSummary
	var received int64
	var receivedTags map[string]int
	if srv := mi.root.peekSigilServer(); srv != nil {
		runs = srv.summaries()
		received, receivedTags = srv.diagnostics()
	}

	var totGen, totTools int
	var totIn, totOut int64
	jsRuns := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		totGen += r.generations
		totTools += len(r.toolCalls)
		totIn += r.inputTokens
		totOut += r.outputTokens

		tools := make([]string, 0, len(r.toolCalls))
		calls := make([]map[string]any, 0, len(r.toolCalls))
		for _, c := range r.toolCalls {
			tools = append(tools, c.Name)
			calls = append(calls, map[string]any{"name": c.Name, "input": c.Input, "output": c.Output})
		}
		jsRuns = append(jsRuns, map[string]any{
			"runId":        r.runID,
			"agent":        r.agent,
			"generations":  r.generations,
			"tools":        tools,
			"toolCalls":    calls,
			"inputTokens":  r.inputTokens,
			"outputTokens": r.outputTokens,
			"output":       r.output,
		})
	}

	mi.logSigilSummary(runs, totGen, totTools, totIn, totOut, received, receivedTags)

	return rt.ToValue(map[string]any{
		"totalRuns":         len(runs),
		"totalGenerations":  totGen,
		"totalToolCalls":    totTools,
		"totalInputTokens":  totIn,
		"totalOutputTokens": totOut,
		"totalReceived":     received,
		"receivedTags":      receivedTags,
		"runs":              jsRuns,
	})
}

// logSigilSummary prints a concise per-run report via the available logger,
// plus a diagnostic hint when generations were received but not routed (or none
// arrived at all).
func (mi *ModuleInstance) logSigilSummary(
	runs []sigilRunSummary, totGen, totTools int, totIn, totOut int64,
	received int64, receivedTags map[string]int,
) {
	log := mi.anyLogger()
	if log == nil {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n=== Sigil data collected ===\n")
	fmt.Fprintf(&sb, "runs=%d generations=%d (received=%d) toolCalls=%d tokens(in/out)=%d/%d\n",
		len(runs), totGen, received, totTools, totIn, totOut)
	for _, r := range runs {
		names := make([]string, 0, len(r.toolCalls))
		for _, c := range r.toolCalls {
			names = append(names, c.Name)
		}
		fmt.Fprintf(&sb, "  [%s] agent=%s gens=%d tokens=%d/%d tools=[%s]\n    output=%s\n",
			r.runID, r.agent, r.generations, r.inputTokens, r.outputTokens,
			strings.Join(names, ", "), truncate(oneLine(r.output), 160))
	}
	switch {
	case received == 0:
		fmt.Fprint(&sb, "\nDIAGNOSTIC: 0 generations reached the ingest endpoint. The agent is not "+
			"exporting to Sigil. Check that it (1) creates a Sigil client that reads SIGIL_ENDPOINT, "+
			"(2) honors SIGIL_INSECURE=true (the endpoint is plaintext gRPC; otherwise the TLS handshake "+
			"fails silently), (3) wraps its model calls in generations, and (4) flushes/Shutdown()s before exit.\n")
	case received > int64(totGen):
		fmt.Fprintf(&sb, "\nDIAGNOSTIC: received %d generations but only %d were routed to runs. "+
			"The test_run_id tag is not matching. test_run_id values seen: %v. "+
			"Ensure SIGIL_TAGS reaches the gRPC export unmodified.\n", received, totGen, receivedTags)
	}
	log.Info(sb.String())
}

// anyLogger returns the VU's logger, falling back to the init-env logger so the
// summary still prints from teardown()/init where VU state may be absent.
func (mi *ModuleInstance) anyLogger() logrus.FieldLogger {
	if st := mi.vu.State(); st != nil && st.Logger != nil {
		return st.Logger
	}
	if ie := mi.vu.InitEnv(); ie != nil && ie.Logger != nil {
		return ie.Logger
	}
	return nil
}

func oneLine(s string) string {
	return strings.NewReplacer("\n", " ", "\r", " ").Replace(s)
}

// sigilServer implements Sigil's GenerationIngestService over gRPC on a local
// ephemeral port, routing each incoming Generation to the per-run collector
// keyed by its test_run_id tag.
type sigilServer struct {
	sigilv1.UnimplementedGenerationIngestServiceServer

	addr string
	gs   *grpc.Server

	mu           sync.Mutex
	runs         map[string]*sigilCollector
	history      []sigilRunSummary
	receivedTags map[string]int // test_run_id value (or sentinel) → generations received
	received     atomic.Int64   // total generations received, regardless of routing
	counter      atomic.Uint64
}

// diagnostics returns the total generations received and a breakdown by the
// test_run_id tag they carried — so a run that collected nothing can be told
// apart from "agent never exported" vs "exported with a mismatched tag".
func (s *sigilServer) diagnostics() (int64, map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tags := make(map[string]int, len(s.receivedTags))
	maps.Copy(tags, s.receivedTags)
	return s.received.Load(), tags
}

// sigilRunSummary is the per-run digest accumulated as runs complete, for the
// end-of-test sigilSummary() report.
type sigilRunSummary struct {
	runID        string
	agent        string
	generations  int
	toolCalls    []ToolCall
	inputTokens  int64
	outputTokens int64
	output       string
}

func (s *sigilServer) recordSummary(rs sigilRunSummary) {
	s.mu.Lock()
	s.history = append(s.history, rs)
	s.mu.Unlock()
}

func (s *sigilServer) summaries() []sigilRunSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]sigilRunSummary, len(s.history))
	copy(out, s.history)
	return out
}

func startSigilServer() (*sigilServer, error) {
	var lc net.ListenConfig
	lis, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("sigil: failed to listen: %w", err)
	}
	s := &sigilServer{
		addr:         lis.Addr().String(),
		runs:         map[string]*sigilCollector{},
		receivedTags: map[string]int{},
	}
	s.gs = grpc.NewServer(grpc.MaxRecvMsgSize(sigilMaxRecvBytes))
	sigilv1.RegisterGenerationIngestServiceServer(s.gs, s)
	go func() { _ = s.gs.Serve(lis) }()
	return s, nil
}

func (s *sigilServer) nextRunID() string {
	n := s.counter.Add(1)
	return fmt.Sprintf("k6-%d-%d", time.Now().UnixNano(), n)
}

func (s *sigilServer) register(id string) *sigilCollector {
	c := &sigilCollector{}
	s.mu.Lock()
	s.runs[id] = c
	s.mu.Unlock()
	return c
}

func (s *sigilServer) deregister(id string) {
	s.mu.Lock()
	delete(s.runs, id)
	s.mu.Unlock()
}

// ExportGenerations implements sigilv1.GenerationIngestServiceServer.
func (s *sigilServer) ExportGenerations(
	_ context.Context, req *sigilv1.ExportGenerationsRequest,
) (*sigilv1.ExportGenerationsResponse, error) {
	gens := req.GetGenerations()
	results := make([]*sigilv1.ExportGenerationResult, 0, len(gens))
	for _, g := range gens {
		s.received.Add(1)
		id := g.GetTags()["test_run_id"]
		tagKey := id
		if tagKey == "" {
			tagKey = "<no test_run_id tag>"
		}
		s.mu.Lock()
		s.receivedTags[tagKey]++
		c := s.runs[id]
		s.mu.Unlock()
		if c != nil {
			c.add(g)
		}
		results = append(results, &sigilv1.ExportGenerationResult{
			GenerationId: g.GetId(),
			Accepted:     true,
		})
	}
	return &sigilv1.ExportGenerationsResponse{Results: results}, nil
}

// sigilCollector accumulates the generations of a single run.
type sigilCollector struct {
	mu   sync.Mutex
	gens []*sigilv1.Generation
	last time.Time
}

func (c *sigilCollector) add(g *sigilv1.Generation) {
	c.mu.Lock()
	c.gens = append(c.gens, g)
	c.last = time.Now()
	c.mu.Unlock()
}

// drain waits until at least one generation has arrived and none has arrived for
// `quiet`, or until `maxWait` elapses, then returns a copy of the collected
// generations.
func (c *sigilCollector) drain(quiet, maxWait time.Duration) []*sigilv1.Generation {
	deadline := time.Now().Add(maxWait)
	for {
		c.mu.Lock()
		n := len(c.gens)
		last := c.last
		c.mu.Unlock()
		if (n > 0 && time.Since(last) >= quiet) || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*sigilv1.Generation, len(c.gens))
	copy(out, c.gens)
	return out
}
