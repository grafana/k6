// Evaluate a recorded LangGraph ReAct-agent run with k6/experimental/ageval.
//
// The agent run is captured once (see capture.py) into trajectory.json — a
// canonical ageval trajectory. This test replays that fixture, so it is fully
// deterministic and CI-safe: no agent provider key is needed, only the LLM judge
// key. Same canonical shape, same assertions as the live Pydantic-AI example —
// proving ageval is framework-agnostic.
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
    name: 'langgraph',
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
