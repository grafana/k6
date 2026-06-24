// Evaluate a REAL agent — Claude Code — in a SINGLE `k6 run`.
//
// ExternalAgent runs the `claude` CLI headless as part of the test, captures its
// trajectory (tool calls + final answer), and scores it. No simulation, no mocks,
// and no separate capture step.
//
//   ANTHROPIC_API_KEY_JUDGE=sk-...  k6 run eval.test.js
//
// (Run it from a directory that contains some .go files — the default task counts
// them. `claude` must be installed and logged in.)
import { check } from 'k6';
import { ExternalAgent, judge } from 'k6/experimental/ageval';

const TASK =
  __ENV.TASK ||
  'Use the Glob tool to find all *.go files in the current directory, then tell me exactly how many there are.';

const claude = new ExternalAgent({
  name: 'claude-code',
  command: 'claude',
  args: [
    '-p',
    '{{input}}',
    '--allowedTools',
    'Glob',
    'Read',
    'LS',
    '--output-format',
    'stream-json',
    '--verbose',
  ],
  format: 'claude-code',
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
  const res = claude.run({ input: TASK, tags: { case: 'count-go-files' } });

  check(res, {
    'used a file-search tool (Glob)': (r) => r.calledTool('Glob'),
    'produced a final answer': (r) => r.output.length > 0,
    'answer contains a number': (r) => /\d/.test(r.output),
  });

  res.expectSequence([{ name: 'Glob' }], { mode: 'in-order', allowOtherCalls: true });

  judge(res, {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The agent was asked to count the *.go files in the current directory. A good answer used a ' +
      'file-search tool and reports a specific, concrete count (a number).',
    threshold: 0.7,
  });
}
