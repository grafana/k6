# k6/experimental/ageval

Evaluate LLM **agents** as part of a `k6` test run — load-testing's reliability tooling,
applied to agentic AI.

`ageval` lets you run an agent (real or simulated) inside a k6 script, assert on **what
it did** (which tools it called, in what order, with what arguments), score **how well it
did** with an LLM-as-judge, and emit the results as first-class k6 metrics. Because it's
just k6, you get the whole k6 machinery for free: thresholds (pass/fail CI gates),
parallel VUs, tags, and every output (cloud dashboards, JSON, Prometheus, …).

```js
import { check } from 'k6';
import { CliAgent, judge } from 'k6/experimental/ageval';

// Drive your REAL agent binary; its stdout is parsed into a gradeable trajectory.
const agent = new CliAgent({
  command: __ENV.AGENT_BIN,
  args: ['eval', '{{input}}'], // {{input}} is replaced with the run() input
  env: { ANTHROPIC_API_KEY: __ENV.ANTHROPIC_API_KEY },
  format: 'claude-code', // built-in output parser (or supply parse: below)
  model: 'claude-haiku-4-5',
});

export default function () {
  const run = agent.run({
    input: 'Type "kubernetes" into the search box and click Search',
    expectedTools: [{ name: 'fillinput' }, { name: 'clickelement' }],
    tags: { case: 'search' },
  });

  // Assert on the trajectory.
  check(run, {
    'used fillinput': (r) => r.callsOf('fillinput').length >= 1,
    'clicked the button': (r) => r.callsOf('clickelement').length >= 1,
  });
  run.expectSequence(); // grades against expectedTools → agent_tool_correctness

  // Score quality with an LLM judge.
  judge(run, {
    name: 'search_quality',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY,
    rubric: 'A good run inspects the page, uses fillinput for the field and clickelement for the button.',
    threshold: 0.7,
  });
}
```

## Core idea: the `AgentTestCase`

Everything centers on one object, the **`AgentTestCase`** — a normalized record of a
single agent run that `check()`, `expectSequence()`, and `judge()` all operate on. You get
one in three ways, depending on how much of the agent you want to run for real:

| Source | What runs | Use when |
| --- | --- | --- |
| **`CliAgent`** | Your **real** agent binary, spawned as a subprocess; its stdout is parsed into a trajectory. | You want to evaluate the actual production agent, not a re-implementation. |
| **`new AgentTestCase({...})`** | Nothing — you supply a trajectory you already have. | You already have a run (an HTTP/API response, a log) and just want to grade it. |
| **`AgentSimulator`** | A built-in agent loop you configure in JS (provider/model/prompt/tools), with mock tool handlers. | You want a fast, self-contained agent defined entirely in the test. |

All three produce the same `AgentTestCase`, so your assertions and judges are identical
regardless of source.

### AgentTestCase API

Fields: `input`, `output`, `toolCalls` (`[{ name, input, output }]`), `expectedTools`,
`usage` (`{ inputTokens, outputTokens }`), `steps`, `duration`.

Methods:
- `calledTool(name)` → bool
- `callsOf(name)` → `[{ name, input, output }]`
- `toolSequence()` → `[string]`
- `stepReports()` / `failedSteps()` → the `report_step` calls (and the failed ones)
- `expectSequence(expected?, opts?)` → grades the actual tool order against an expected
  sequence and emits `agent_tool_correctness`. With no argument it uses the
  `expectedTools` attached at `run()` time.

## `CliAgent` — evaluate your real agent

Spawns your real agent as a subprocess and turns its output into an `AgentTestCase`, so the
agent runs as part of a single `k6 run` — no separate capture step, and you grade the
actual production agent rather than a re-implementation.

```js
import { CliAgent } from 'k6/experimental/ageval';

const agent = new CliAgent({
  command: __ENV.AGENT_BIN,        // the binary to run
  args: ['eval', '{{input}}'],     // {{input}} is replaced with run() input; else input goes to stdin
  env: { ANTHROPIC_API_KEY: __ENV.ANTHROPIC_API_KEY },
  format: 'claude-code',           // built-in output parser; or use parse: below
  // parse: (stdout) => ({ output, toolCalls: [{name, input, output}], usage: {inputTokens, outputTokens} }),
  model: 'claude-haiku-4-5',       // label only (tags the metrics)
  timeoutSeconds: 300,
});

const run = agent.run({ input: 'Do the thing', expectedTools: [{ name: 'fillinput' }] });
```

Output parsing is either a built-in **`format`** adapter or a custom **`parse(stdout)`**
callback. A `parse` callback (and `new AgentTestCase({...})`, below) expects this shape:

```json
{ "output": "...", "toolCalls": [{ "name": "...", "input": {}, "output": "..." }],
  "usage": { "inputTokens": 0, "outputTokens": 0 } }
```

Built-in `format` adapters: `claude-code`, `codex`, `a2a`, `openai`, `anthropic`.

## `AgentTestCase` — grade a trajectory you already have

When the agent ran somewhere else (you have its response from an HTTP/API call, a log, or a
previous step), build an `AgentTestCase` directly from the trajectory and grade it — no
agent is run.

```js
import http from 'k6/http';
import { check } from 'k6';
import { AgentTestCase, judge } from 'k6/experimental/ageval';

export default function () {
  const res = http.post('https://my-agent.internal/run', JSON.stringify({ task: '...' }));

  // Adapt your API response into the trajectory shape, or pass a raw body + `format`.
  const run = new AgentTestCase({
    input: '...',
    output: res.json('answer'),
    toolCalls: res.json('tool_calls'), // [{ name, input, output }]
    usage: { inputTokens: res.json('usage.in'), outputTokens: res.json('usage.out') },
    expectedTools: [{ name: 'search' }],
  });

  check(run, { 'searched first': (r) => r.toolSequence()[0] === 'search' });
  run.expectSequence();
  judge(run, { name: 'answer_quality', model: 'claude-haiku-4-5', apiKey: __ENV.ANTHROPIC_API_KEY, rubric: '...' });
}
```

## `AgentSimulator` — a self-contained agent in the test

When you don't have a separate agent to run, `AgentSimulator` is a built-in agent loop you
configure entirely in JS (provider/model/prompt/tools) with mock tool handlers.

```js
import { AgentSimulator } from 'k6/experimental/ageval';

const agent = new AgentSimulator({
  provider: 'anthropic',     // default 'anthropic' (only provider today)
  model: 'claude-haiku-4-5', // required; see Supported models
  apiKey: __ENV.ANTHROPIC_API_KEY,
  systemPrompt: '...',
  maxSteps: 30,              // default 30
  name: 'agent',             // tag value; default 'agent'
  baseURL: '...',            // optional, override the API endpoint
  tools: [
    {
      name: 'fillinput',
      description: 'Fill a form field',
      inputSchema: { type: 'object', properties: { selector: { type: 'string' }, text: { type: 'string' } } },
      mock: (input) => `Input filled: ${input.selector}`, // what the tool "returns"
    },
    // ...
  ],
});

const run = agent.run({ input: 'Do the thing', expectedTools: [{ name: 'fillinput' }] });
```

`agent.run({ input, mocks?, expectedTools?, tags? })` runs the loop and returns an
`AgentTestCase`. `mocks` lets a single run override a tool's `mock` handler.

## `judge()`

LLM-as-judge (GEval-style): scores an `AgentTestCase` against a natural-language rubric on
a 0..1 scale, returns `{ score, reason, passed }`, and emits the quality metrics.

```js
const verdict = judge(run, {
  name: 'search_quality',          // REQUIRED — the eval label (emitted as the `eval` tag)
  provider: 'anthropic',
  model: 'claude-haiku-4-5',       // required
  apiKey: __ENV.ANTHROPIC_API_KEY, // required
  rubric: 'A good run ...',        // required
  threshold: 0.7,                  // default 0.7; score >= threshold → passed
  input: '...',                    // optional, overrides what the judge sees
  actualOutput: '...',             // optional
});
```

`name` is mandatory because the rubric and reason are not metrics — `name` is what makes
each eval identifiable in dashboards and downstream tooling. The judge also logs a
parseable `ageval_eval {json}` line carrying the name/score/passed/reason.

## Metrics

All metrics are tagged with `eval` (the judge `name`), `threshold`, the agent/judge
`model`, plus any `tags` you pass to `run()` (e.g. `case`). Tool/usage metrics add `tool`
and `direction` tags.

| Metric | Type | Meaning |
| --- | --- | --- |
| `agent_duration` | Trend (time) | wall-clock per run |
| `agent_steps` | Trend | LLM round-trips |
| `agent_tool_calls` | Counter | tool invocations (tag: `tool`) |
| `agent_tokens` | Counter | agent token usage (tag: `direction`=input/output) |
| `agent_cost_usd` | Counter | agent spend |
| `agent_tool_correctness` | Rate | `expectSequence()` pass rate |
| `agent_quality_score` | Trend | judge score (0..1) |
| `agent_judge_pass` | Rate | judge pass rate (score ≥ threshold) |
| `agent_judge_tokens` | Counter | judge token usage |
| `agent_judge_cost_usd` | Counter | judge spend |

Gate them in CI with k6 thresholds:

```js
export const options = {
  thresholds: {
    checks: ['rate>0.9'],
    agent_quality_score: ['avg>0.7'],
    agent_judge_pass: ['rate>0.9'],
    agent_tool_correctness: ['rate>0.9'],
  },
};
```

## Supported models

Anthropic (provider `anthropic`): `claude-opus-4-8`, `claude-opus-4-7`,
`claude-sonnet-4-6`, `claude-sonnet-4-5`, `claude-haiku-4-5`, `claude-fable-5`. Token
costs are built in, so `agent_cost_usd` / `agent_judge_cost_usd` are computed for you.

## Status

Experimental. The module is registered as `k6/experimental/ageval`; the API may change.
