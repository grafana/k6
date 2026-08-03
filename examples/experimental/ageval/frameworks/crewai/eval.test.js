// Evaluate a recorded CrewAI MULTI-AGENT crew with k6/experimental/ageval.
//
// The crew (Researcher -> Writer, on Claude) is captured once by capture.py into
// trajectory.json. The trajectory spans BOTH agents' tool calls, so this shows
// ageval grading a multi-agent / sub-agent pipeline: the Researcher must look the
// company up (lookup_company) BEFORE the Writer publishes (publish_summary), and
// the final summary must contain the researched facts. Deterministic / CI-safe;
// only the LLM judge key is needed.
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
    name: 'crewai',
    input: run.input,
    output: run.output,
    toolCalls: run.toolCalls,
    usage: run.usage,
    model: run.model,
    // Cross-agent ordering: research must precede publishing.
    expectedTools: [{ name: 'lookup_company' }, { name: 'publish_summary' }],
    tags: { case: 'multi_agent_summary' },
  });

  check(res, {
    'researcher looked up the company': (r) => r.calledTool('lookup_company'),
    'writer published a summary': (r) => r.calledTool('publish_summary'),
    'summary cites the industry': (r) => /widget/i.test(r.output), // 'widget' or 'widgets'
    'summary cites the founding year': (r) => /1998/.test(r.output),
  });

  // No argument → grade against res.expectedTools (lookup_company before publish_summary).
  res.expectSequence();

  judge(res, {
    name: 'company_summary',
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'A two-agent crew was asked to research Acme Corp and publish a one-sentence summary. A good ' +
      'result is a single concise sentence that correctly states Acme Corp is a widgets company ' +
      'founded in 1998. Penalize invented facts, missing facts, or multi-sentence rambling.',
    threshold: 0.7,
  });
}
