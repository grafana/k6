// Demo of the "real agent" mode of k6/experimental/ageval.
//
// Instead of simulating an agent (see self_contained.js, which uses
// AgentSimulator + mocked tools), here you feed in a REAL agent's recorded
// trajectory — its final output and the tools it actually called — and evaluate
// it directly. No agent model call happens; only the LLM-as-judge runs.
//
// This is how you wire ageval into your production agent: run your agent however
// you like (your app, your framework, a captured transcript), collect its output
// + tool calls, and assert/score them here. Every run still emits the standard
// agent_* metrics for k6 Cloud / Grafana.
//
//   ANTHROPIC_API_KEY_JUDGE=sk-...  k6 run real_agent.js
import { check } from 'k6';
import { fromAgentRun, judge } from 'k6/experimental/ageval';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    agent_tool_correctness: ['rate>0.9'],
    agent_quality_score: ['avg>0.7'],
    agent_judge_pass: ['rate>0.9'],
  },
};

// Pretend this is what your real agent produced for the request
// "Was invoice INV-123 for alice@example.com paid?". In practice you'd load this
// from your agent's run (an API response, a logged transcript, a SharedArray of
// captured cases, etc.).
const recordedRun = {
  model: 'claude-haiku-4-5', // optional: enables token/cost metrics
  output: 'Invoice INV-123 is paid ($99 USD).',
  toolCalls: [
    { name: 'get_customer', input: { email: 'alice@example.com' }, output: '{"id":"cust_123","plan":"Pro"}' },
    { name: 'get_invoice', input: { invoice_id: 'INV-123' }, output: '{"status":"paid","amount":99}' },
  ],
  usage: { inputTokens: 1500, outputTokens: 120 }, // optional
  durationMs: 5300, // optional
  tags: { case: 'invoice_paid', source: 'production' },
};

export default function () {
  const res = fromAgentRun(recordedRun);

  check(res, {
    'called get_customer': (r) => r.calledTool('get_customer'),
    'called get_invoice': (r) => r.calledTool('get_invoice'),
    'mentions paid': (r) => /paid/i.test(r.output),
    'hides internal id': (r) => !/cust_123/.test(r.output),
  });

  res.expectSequence(
    [{ name: 'get_customer' }, { name: 'get_invoice', args: { invoice_id: 'INV-123' } }],
    { mode: 'in-order', allowOtherCalls: true },
  );

  judge(res, {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The answer must state the invoice is paid, must not expose the internal customer id ' +
      '(cust_123), and must be concise.',
    threshold: 0.7,
  });
}
