# SigilAgent — evaluating a Sigil-instrumented agent

`SigilAgent` is an ageval producer that runs a real agent as a subprocess and
builds its `AgentTestCase` from the Grafana AI observability (Sigil) telemetry
the agent streams while it runs — instead of parsing stdout like
`CliAgent`.

This means **one instrumentation serves both production observability and
evaluation**: if your agent already emits Sigil generations in prod, ageval can
ingest the same stream to grade it.

## How it works

```
SigilAgent.run({input})                  ageval gRPC server (one per k6 process)
  ├─ assign a unique test_run_id          implements Sigil GenerationIngestService
  ├─ spawn the agent with env:            routes each Generation by tags[test_run_id]
  │    SIGIL_ENDPOINT=127.0.0.1:<port>
  │    SIGIL_PROTOCOL=grpc
  │    SIGIL_INSECURE=true
  │    SIGIL_TAGS=test_run_id=<id>    ◄── agent streams Generations during the run
  │    SIGIL_CONTENT_CAPTURE_MODE=full
  ├─ pipe the task to stdin (or {{input}})
  ├─ wait for exit (+ brief flush grace)
  └─ map Generations → trajectory → AgentTestCase
```

The `Generation` stream is reduced to the normalized trajectory:

- **tool calls** ← assistant output `ToolCall` parts (matched to their
  `ToolResult` parts by `tool_call_id`),
- **output** ← the final assistant text part,
- **usage** ← summed `TokenUsage` (input + cache + output),
- **steps** ← number of generations (model round-trips).

Everything downstream — `check`, `expectSequence`, `judge`, and the `agent_*`
metrics — is identical to every other ageval producer.

## Agent-side contract

Your agent must:

1. **Be instrumented with a Sigil SDK** ([Go][go-sdk] / [Python][py-sdk] /
   [JS][js-sdk]). The SDK reads the `SIGIL_*` env vars on construction, so you
   typically just `NewClient()` with no explicit config.
2. **Read its task from stdin** (default), or accept a `{{input}}` placeholder in
   `args` that ageval substitutes.
3. **Flush before exit** — call the SDK's `Shutdown()` (or equivalent) so the
   final batched generations are sent before the process ends. ageval waits a
   short grace period after exit, but an agent that exits without flushing may
   lose its last generations.

You do **not** set `SIGIL_TAGS` / `SIGIL_ENDPOINT` yourself — ageval injects them
per run. Any `env` you pass to `SigilAgent` is merged in (the `SIGIL_*` wiring
wins on conflicts).

## Config

```js
new SigilAgent({
  command: './myagent',     // required
  args: ['run'],            // optional; include '{{input}}' to pass the task as an arg
  env: { FOO: 'bar' },      // optional extra env, merged with the SIGIL_* wiring
  cwd: '.',                 // optional working directory
  model: 'claude-haiku-4-5',// optional; enables agent_cost_usd via the pricing table
  name: 'myagent',          // optional; tags metrics + sets SIGIL_AGENT_NAME
  stepReportTool: 'report_step', // optional; tool name for failedSteps()
  timeoutSeconds: 120,      // optional; subprocess timeout (default 300)
});
```

[sigil]: https://grafana.com/docs/grafana-cloud/machine-learning/ai-observability/
[go-sdk]: https://github.com/grafana/sigil-sdk/tree/main/go
[py-sdk]: https://github.com/grafana/sigil-sdk/tree/main/python
[js-sdk]: https://github.com/grafana/sigil-sdk/tree/main/js
