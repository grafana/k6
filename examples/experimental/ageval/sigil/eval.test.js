// Eval driven through a REAL, Sigil-instrumented agent.
//
// Unlike CliAgent (which parses the agent's stdout), SigilAgent collects the
// trajectory from the Grafana AI observability (Sigil) telemetry the agent
// streams while it runs. ageval starts a local gRPC ingest endpoint, points the
// agent at it via SIGIL_* env vars, and correlates the stream to each run with a
// unique `test_run_id` tag — so it works under k6 VUs/iterations.
//
// The agent under test must:
//   - be instrumented with a Sigil SDK (it reads the SIGIL_* env vars on its own),
//   - read its task from stdin (or use a `{{input}}` placeholder in `args`),
//   - flush the Sigil client before exiting (e.g. client.Shutdown()).
//
// Run with:
//   AGENT_BIN=./myagent ANTHROPIC_API_KEY_JUDGE=sk-... k6 run eval.test.js

import { check } from 'k6';
import { SigilAgent, judge, sigilSummary } from 'k6/experimental/ageval';

const AGENT_BIN = __ENV.AGENT_BIN || './myagent';
const MODEL = __ENV.AGENT_MODEL || 'claude-haiku-4-5';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate>0.9'],
    agent_tool_correctness: ['rate>0.9'],
    agent_judge_pass: ['rate>0.9'],
  },
};

const agent = new SigilAgent({
  command: AGENT_BIN,
  args: ['run'], // no {{input}} → the task is piped to the agent's stdin
  name: 'myagent',
  model: MODEL, // enables agent_cost_usd when the model is in ageval's pricing table
  timeoutSeconds: 120,
});

export default function () {
  const run = agent.run({
    input: 'What is the weather in Paris? Use the available tools.',
    expectedTools: [{ name: 'get_weather' }],
    tags: { case: 'weather' },
  });

  // run.toolCalls / run.output were reconstructed from the agent's Sigil
  // generations — the assertion API is identical to every other producer.
  check(run, {
    'called the weather tool': (r) => r.calledTool('get_weather'),
    'answered': (r) => r.output.length > 0,
  });

  run.expectSequence(); // grades against expectedTools → agent_tool_correctness

  judge(run, {
    name: 'weather_answer',
    provider: 'anthropic',
    model: MODEL,
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric: 'The answer must state the weather in Paris based on the tool result, concisely.',
    threshold: 0.7,
  });
}

// Runs once at the end: prints a report of all Sigil data collected this test
// (per-run generations, tool calls, tokens, final output) and returns the
// aggregate, here logged as JSON too.
export function teardown() {
  const summary = sigilSummary();
  console.log('Sigil summary: ' + JSON.stringify(summary));
}