// Evaluate a REAL agent — OpenAI Codex CLI — in a SINGLE `k6 run`.
//
// ExternalAgent runs the `codex` CLI non-interactively (`codex exec --json`) as
// part of the test, captures its trajectory (shell/tool calls + final answer) via
// the built-in `codex` adapter, and scores it. No simulation, no mocks, and no
// separate capture step.
//
//   ANTHROPIC_API_KEY_JUDGE=sk-...  k6 run eval.test.js
//
// (Run it from a directory that contains some .go files — the default task counts
// them. `codex` must be installed and logged in: `codex login`.)
import { check } from 'k6';
import { ExternalAgent, judge } from 'k6/experimental/ageval';

const TASK =
  __ENV.TASK ||
  'How many .go files are in the current directory? Use a shell command to list them, then state the number.';

const codex = new ExternalAgent({
  name: 'codex',
  command: 'codex',
  args: [
    'exec',
    '--json',
    '--skip-git-repo-check',
    '--sandbox',
    'read-only',
    '{{input}}',
  ],
  format: 'codex',
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
  const res = codex.run({ input: TASK, tags: { case: 'count-go-files' } });

  check(res, {
    'ran a shell command': (r) => r.calledTool('shell'),
    'produced a final answer': (r) => r.output.length > 0,
    'answer contains a number': (r) => /\d/.test(r.output),
  });

  res.expectSequence([{ name: 'shell' }], { mode: 'in-order', allowOtherCalls: true });

  judge(res, {
    provider: 'anthropic',
    model: 'claude-haiku-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The agent was asked to count the *.go files in the current directory. A good answer ran a ' +
      'shell command to list them and reports a specific, concrete count (a number).',
    threshold: 0.7,
  });
}
