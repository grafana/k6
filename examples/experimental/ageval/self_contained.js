// Self-contained demo of the experimental k6/experimental/ageval module.
//
// It runs a small "support agent" against the Anthropic API with two mocked
// tools, asserts on the tool-call trajectory with check() and expectSequence(),
// and scores the final answer with an LLM-as-judge. Every run emits standard k6
// metrics (agent_duration, agent_tokens, agent_cost_usd, agent_tool_correctness,
// agent_quality_score, agent_judge_pass) that visualize in k6 Cloud / Grafana.
//
// Run it with:
//   ANTHROPIC_API_KEY=sk-...  ANTHROPIC_API_KEY_JUDGE=sk-...  k6 run self_contained.js
import { check } from 'k6';
import { AgentSimulator, judge } from 'k6/experimental/ageval';

export const options = {
  vus: 1,
  iterations: 2,
  thresholds: {
    agent_tool_correctness: ['rate>0.9'],
    agent_quality_score: ['avg>0.6'],
    agent_judge_pass: ['rate>0.5'],
    agent_duration: ['p(95)<30000'],
    agent_cost_usd: ['count<1'],
  },
};

const getCustomer = {
  name: 'get_customer',
  description: 'Look up a customer by email. Returns id, email and plan.',
  inputSchema: {
    type: 'object',
    properties: { email: { type: 'string', description: 'Customer email' } },
    required: ['email'],
  },
  mock: (input) => JSON.stringify({ id: 'cust_123', email: input.email, plan: 'Pro' }),
};

const getInvoice = {
  name: 'get_invoice',
  description: 'Look up an invoice by its id. Returns status, amount and currency.',
  inputSchema: {
    type: 'object',
    properties: { invoice_id: { type: 'string', description: 'Invoice id, e.g. INV-123' } },
    required: ['invoice_id'],
  },
  mock: (input) => JSON.stringify({ invoice_id: input.invoice_id, status: 'paid', amount: 99, currency: 'USD' }),
};

const supportAgent = new AgentSimulator({
  provider: 'anthropic',
  model: 'claude-opus-4-8',
  apiKey: __ENV.ANTHROPIC_API_KEY,
  systemPrompt:
    'You are a billing support agent. Use the available tools to answer the user. ' +
    'Look up the customer, then the invoice. Be concise, state whether the invoice is paid, ' +
    'and never expose the internal customer id.',
  tools: [getCustomer, getInvoice],
});

export default function () {
  const res = supportAgent.run({
    input: 'Was invoice INV-123 for alice@example.com paid?',
    expectedTools: [{ name: 'get_customer' }, { name: 'get_invoice' }],
    tags: { case: 'invoice_paid' },
  });

  check(res, {
    'called get_customer': (r) => r.calledTool('get_customer'),
    'called get_invoice': (r) => r.calledTool('get_invoice'),
    'mentions paid': (r) => /paid/i.test(r.output),
    'hides internal id': (r) => !/cust_123/.test(r.output),
  });

  // No argument → grade the trajectory against res.expectedTools (in-order).
  res.expectSequence();

  judge(res, {
    provider: 'anthropic',
    model: 'claude-opus-4-8',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The answer must be concise, must say the invoice is paid, must not expose the internal ' +
      'customer id (cust_123), and must not invent next steps or information not provided by tools.',
    threshold: 0.7,
  });
}
