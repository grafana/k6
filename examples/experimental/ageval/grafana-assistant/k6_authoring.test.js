// Eval the Grafana Assistant's "k6 Script Authoring" behavior over its local
// A2A REST endpoint, using k6/experimental/ageval.
//
// k6 calls the assistant directly (POST /api/v1/a2a, A2A/JSON-RPC, SSE), parses
// the streamed trajectory (step.toolCall / step.message / step.complete) into the
// ageval shape, then grades it with AgentTestCase + check/expectSequence/judge —
// emitting the standard agent_* metrics. Because it runs in k6, you can also run
// it under VUs/ramping to measure the assistant's quality + latency/cost under
// concurrency.
//
// Target the llmspec env (local auth, seeded terms/creds), NOT bare `mise run up`:
//   mise run llmspec:env:up          # assistant A2A on http://localhost:9191
//
// Run:
//   ANTHROPIC_API_KEY_JUDGE=sk-... \
//   A2A_URL=http://localhost:9191/api/v1/a2a \
//   GRAFANA_TOKEN=<sa-or-key>  ORG_ID=1  GRAFANA_USER=test@grafana.com \
//   k6 run k6_authoring.test.js
import http from 'k6/http';
import { check } from 'k6';
import encoding from 'k6/encoding';
import { AgentTestCase, judge } from 'k6/experimental/ageval';

const A2A_URL = __ENV.A2A_URL || 'http://localhost:9191/api/v1/a2a';
const TASK =
  __ENV.TASK ||
  'Write a k6 load test script for https://api.example.com/users with 10 virtual users for 1 minute, ' +
    'including a check that the response status is 200 and a p95 latency threshold. Just generate it directly.';

// Grafana credential. The local llmspec env uses HTTP Basic admin:admin
// (llmspec's `--grafana-api-key=basic:admin:admin`); a service-account token is
// sent as Bearer instead. Mirror llmspec's applyGrafanaAuth.
const GRAFANA_TOKEN = __ENV.GRAFANA_TOKEN || 'basic:admin:admin';
function authHeaders() {
  if (GRAFANA_TOKEN.indexOf('basic:') === 0) {
    return { Authorization: 'Basic ' + encoding.b64encode(GRAFANA_TOKEN.slice(6)) };
  }
  return { Authorization: `Bearer ${GRAFANA_TOKEN}`, 'X-Grafana-API-Key': GRAFANA_TOKEN };
}

const HEADERS = Object.assign(
  {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
    'X-App-Source': 'assistant',
    'X-Scope-OrgID': __ENV.ORG_ID || '1',
    'X-Grafana-URL': __ENV.GRAFANA_URL || 'http://grafana:3000',
    'X-Grafana-User': __ENV.GRAFANA_USER || 'admin',
    'X-Grafana-User-ID': __ENV.GRAFANA_USER_ID || '1',
  },
  authHeaders(),
);

// setup accepts the Assistant terms & conditions once for this tenant/identity
// (the gate that otherwise returns 403 TERMS_NOT_ACCEPTED).
export function setup() {
  const url = A2A_URL.replace(/\/a2a$/, '/settings/accept-terms');
  const res = http.put(url, JSON.stringify({ acceptedTermsAndConditions: true }), {
    headers: Object.assign({}, HEADERS, { Accept: 'application/json' }),
  });
  console.log(`accept-terms: HTTP ${res.status}`);
}

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate>0.9'],
    agent_quality_score: ['avg>0.7'],
    agent_judge_pass: ['rate>0.9'],
  },
};

// callAssistant POSTs an A2A message/stream request and returns the raw SSE body.
function callAssistant(task) {
  const payload = JSON.stringify({
    jsonrpc: '2.0',
    id: '1',
    method: 'message/stream',
    params: {
      message: {
        kind: 'message',
        role: 'user',
        messageId: `k6-${__VU}-${__ITER}`,
        parts: [{ kind: 'text', text: task }],
      },
    },
  });
  const res = http.post(A2A_URL, payload, { headers: HEADERS, timeout: '180s' });
  return res.body || '';
}

// parseA2A turns the A2A SSE stream into the ageval trajectory shape.
function parseA2A(body) {
  const toolCalls = [];
  const byId = {};
  let output = '';
  let usage = { inputTokens: 0, outputTokens: 0 };

  for (const line of body.split('\n')) {
    const s = line.trim();
    if (!s.startsWith('data:')) continue;
    let ev;
    try {
      ev = JSON.parse(s.slice(5).trim());
    } catch (_) {
      continue;
    }
    const r = ev.result;
    if (!r) continue;

    if (r.kind === 'artifact-update' && r.artifact) {
      const art = r.artifact;
      const parts = art.parts || [];
      if (art.name === 'step.toolCall') {
        for (const p of parts) {
          if (p.kind === 'data' && p.data && p.data.toolName) {
            const tc = { _id: p.data.toolId, name: p.data.toolName, input: p.data.inputs || {}, output: '' };
            toolCalls.push(tc);
            if (p.data.toolId) byId[p.data.toolId] = tc;
          }
        }
      } else if (art.name === 'step.message') {
        for (const p of parts) {
          if (p.kind === 'text' && p.text) output += p.text;
        }
      } else if (art.name === 'step.toolResult') {
        for (const p of parts) {
          if (p.kind === 'data' && p.data && byId[p.data.toolId]) {
            const v = p.data.result;
            byId[p.data.toolId].output = typeof v === 'string' ? v : JSON.stringify(v);
          }
        }
      } else if (art.name === 'step.complete' && art.metadata && art.metadata['agent-traceability']) {
        const u = art.metadata['agent-traceability'].usage || {};
        usage.inputTokens += u.inputTokens || 0;
        usage.outputTokens += u.outputTokens || 0;
      }
    } else if (r.kind === 'status-update' && r.status && r.status.message && r.status.message.parts) {
      // Final assistant message (final answer, or a failure reason).
      for (const p of r.status.message.parts) {
        if (p.kind === 'text' && p.text) output = p.text;
      }
    }
  }
  return { output, toolCalls, usage };
}

export default function () {
  const body = callAssistant(TASK);
  const run = parseA2A(body);

  const res = new AgentTestCase({
    name: 'grafana-assistant',
    input: TASK,
    output: run.output,
    toolCalls: run.toolCalls,
    usage: run.usage,
    tags: { case: 'k6-authoring' },
  });

  // Print the trajectory so you can see what the assistant produced.
  console.log(`\n===== tool calls (${run.toolCalls.length}): ${run.toolCalls.map((c) => c.name).join(', ') || '(none)'}`);
  console.log(`===== assistant output (the generated k6 script) =====\n${res.output}\n===== end output =====\n`);

  check(res, {
    'produced a response': (r) => r.output.length > 0,
    // k6_script_handler is a frontend/browser tool; over backend REST it may not
    // execute, so we accept either the tool call OR an inline script in the text.
    'authored a k6 script (tool or text)': (r) =>
      r.calledTool('k6_script_handler') || /export\s+default\s+function/.test(r.output),
    'script imports k6/http': (r) =>
      r.calledTool('k6_script_handler') || /k6\/http/.test(r.output),
    'includes thresholds': (r) => r.calledTool('k6_script_handler') || /thresholds/.test(r.output),
  });

  judge(res, {
    name: 'k6_authoring',
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The user asked the Grafana Assistant to author a k6 load test (10 VUs, 1 minute, a status==200 ' +
      'check, and a p95 latency threshold). A good response produces a complete, runnable k6 script — ' +
      'export default function, an http request, a check(), and an options.thresholds block — matching ' +
      'the requested load. Penalize refusals, non-k6 output, or scripts missing checks/thresholds.',
    threshold: 0.7,
  });
}
