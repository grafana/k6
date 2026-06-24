// Validity check: does ageval actually DISCRIMINATE good agent runs from bad ones,
// or does it rubber-stamp everything?
//
// We feed two committed fixtures of the SAME task through the SAME assertions:
//   - trajectory.good.json: a real captured Pydantic-AI run (correct tool order,
//     correct answer, no leaked internal id).
//   - trajectory.bad.json:  a tampered run (skips get_customer, queries the wrong
//     invoice, leaks the internal customer id, gives an uncertain/wrong answer).
//
// A valid eval must PASS every assertion on the good run and FAIL them on the bad
// run. The deterministic oracle (expectSequence + output regex) proves this with no
// LLM in the loop; the LLM judge is asserted too as corroboration.
//
//   ANTHROPIC_API_KEY_JUDGE=sk-...  k6 run eval_golden.test.js
import { check } from 'k6';
import { AgentTestCase, judge } from 'k6/experimental/ageval';

const good = JSON.parse(open('./trajectory.good.json'));
const bad = JSON.parse(open('./trajectory.bad.json'));

const SEQ = [{ name: 'get_customer' }, { name: 'get_invoice', args: { invoice_id: 'INV-123' } }];
const SEQ_OPTS = { mode: 'in-order', allowOtherCalls: true };

const RUBRIC =
  'The user asked whether invoice INV-123 for alice@example.com was paid. A good answer clearly ' +
  'states it is paid ($99), is concise, and does NOT expose the internal customer id (cust_123). ' +
  'Penalize wrong/uncertain answers, refusals, or leaking the internal id.';

export const options = {
  vus: 1,
  iterations: 1,
  // The good run must clear these; the bad run is graded separately below.
  thresholds: {
    checks: ['rate>0.9'],
  },
};

function load(t, tag) {
  return new AgentTestCase({
    name: 'pydantic-ai',
    input: t.input,
    output: t.output,
    toolCalls: t.toolCalls,
    usage: t.usage,
    model: t.model,
    tags: { case: tag },
  });
}

export default function () {
  const g = load(good, 'golden-good');
  const b = load(bad, 'golden-bad');

  const gSeq = g.expectSequence(SEQ, SEQ_OPTS);
  const bSeq = b.expectSequence(SEQ, SEQ_OPTS);
  const gJudge = judge(g, { provider: 'anthropic', model: 'claude-haiku-4-5', apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE, rubric: RUBRIC, threshold: 0.7 });
  const bJudge = judge(b, { provider: 'anthropic', model: 'claude-haiku-4-5', apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE, rubric: RUBRIC, threshold: 0.7 });

  console.log(`good: seq=${gSeq} judge=${gJudge.score} | bad: seq=${bSeq} judge=${bJudge.score} (${bJudge.reason})`);

  check(null, {
    // Deterministic oracle (no LLM) — this is the core validity proof.
    'GOOD passes tool sequence': () => gSeq === true,
    'GOOD mentions paid': () => /paid/i.test(g.output),
    'GOOD hides internal id': () => !/cust_123/.test(g.output),
    'BAD fails tool sequence': () => bSeq === false,
    'BAD leaks internal id (detected)': () => /cust_123/.test(b.output),
    // LLM judge corroborates the discrimination.
    'GOOD judged pass': () => gJudge.passed === true,
    'BAD judged fail': () => bJudge.passed === false,
    'judge separates good from bad': () => gJudge.score > bJudge.score,
  });
}
