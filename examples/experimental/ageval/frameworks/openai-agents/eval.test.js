// Evaluate a recorded OpenAI Agents SDK run with k6/experimental/ageval.
//
// The agent (run on Claude via the SDK's LiteLLM extension) is captured once by
// capture.py into trajectory.json — a canonical ageval trajectory. This test
// replays that fixture, so it is deterministic and CI-safe (only the LLM judge
// key is needed). Same canonical shape and assertions as the other frameworks.
//
//   ANTHROPIC_API_KEY_JUDGE=sk-ant-...  k6 run eval.test.js
import { check } from 'k6';
import { AgentTestCase, judge } from 'k6/experimental/ageval';

const run = JSON.parse(open('./trajectory.json'));

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
  const res = new AgentTestCase({
    name: 'openai-agents',
    input: run.input,
    output: run.output,
    toolCalls: run.toolCalls,
    usage: run.usage,
    model: run.model,
    expectedTools: [{ name: 'get_customer' }, { name: 'get_invoice', input: { invoice_id: 'INV-123' } }],
    tags: { case: 'invoice_paid' },
  });

  check(res, {
    'called get_customer': (r) => r.calledTool('get_customer'),
    'called get_invoice': (r) => r.calledTool('get_invoice'),
    'mentions paid': (r) => /paid/i.test(r.output),
    'hides internal id': (r) => !/cust_123/.test(r.output),
  });

  // No argument → grade against res.expectedTools.
  res.expectSequence();

  judge(res, {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The user asked whether invoice INV-123 for alice@example.com was paid. A good answer states ' +
      'the invoice is paid ($99), is concise, and does NOT expose the internal customer id (cust_123).',
    threshold: 0.7,
  });
}
