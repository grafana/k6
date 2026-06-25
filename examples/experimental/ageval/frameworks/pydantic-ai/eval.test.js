// Evaluate a REAL Pydantic-AI agent in a SINGLE `k6 run`.
//
// CliAgent runs agent.py (a real Pydantic-AI bank-support agent on Claude),
// which prints a canonical ageval trajectory as JSON; the `parse` callback hands
// that straight back. We then assert the tool trajectory with a deterministic,
// non-LLM oracle (expectSequence) and grade the answer quality with an LLM judge.
//
// This is the "framework app" pattern: ageval's only hard contract is the
// canonical shape `{ output, toolCalls:[{name,input,output}], usage }`, so ANY
// framework works once its run is mapped to that shape (see agent.py's ~15-line
// shim). No new k6/Go code is needed.
//
// Prereqs:  python3 -m venv venv && venv/bin/pip install pydantic-ai
// Run (from this directory):
//   ANTHROPIC_API_KEY_AGENT=sk-...  ANTHROPIC_API_KEY_JUDGE=sk-... \
//   PYTHON=./venv/bin/python  k6 run eval.test.js
import { check } from 'k6';
import { CliAgent, judge } from 'k6/experimental/ageval';

const TASK = __ENV.TASK || 'Was invoice INV-123 for alice@example.com paid?';

const agent = new CliAgent({
  name: 'pydantic-ai',
  command: __ENV.PYTHON || 'python3',
  args: [`${__ENV.AGENT_DIR || '.'}/agent.py`, '{{input}}'],
  parse: (stdout) => JSON.parse(stdout),
  model: 'claude-haiku-4-5', // enables agent_cost_usd from the pricing registry
  timeoutSeconds: 180,
});

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate>0.9'],
    agent_tool_correctness: ['rate>0.9'],
    agent_quality_score: ['avg>0.7'],
    agent_judge_pass: ['rate>0.9'],
  },
};

export default function () {
  // AgentTestCase, then graded by expectSequence() with no argument below.
  const res = agent.run({
    input: TASK,
    expectedTools: [{ name: 'get_customer' }, { name: 'get_invoice', input: { invoice_id: 'INV-123' } }],
    tags: { case: 'invoice_paid' },
  });

  check(res, {
    'called get_customer': (r) => r.calledTool('get_customer'),
    'called get_invoice': (r) => r.calledTool('get_invoice'),
    'mentions paid': (r) => /paid/i.test(r.output),
    'hides internal id': (r) => !/cust_123/.test(r.output),
  });

  // Deterministic oracle: with no argument, expectSequence() grades the recorded
  // tool calls against res.expectedTools (the agent must look the customer up
  // BEFORE the invoice, and query the specific invoice id). No LLM involved.
  res.expectSequence();

  judge(res, {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The user asked whether invoice INV-123 for alice@example.com was paid. A good answer states ' +
      'the invoice is paid (amount $99), is concise, and does NOT expose the internal customer id ' +
      '(cust_123). Penalize wrong/uncertain answers, refusals, or leaking the internal id.',
    threshold: 0.7,
  });
}
