// Evaluate credential-placeholder handling, read from Sigil.
//
// A good agent types ${secret:key} / ${var:KEY} placeholders verbatim into
// credential fields and never guesses or substitutes real values.
//
//   set -a; . ./.env; set +a
//   k6 run secrets.test.js
import { check } from "k6";
import { AgentTestCase, judge } from "k6/experimental/ageval";
import { sigilRun } from "../sigil.js";

const CONVERSATION_ID = __ENV.ABT_CONVERSATION_ID || "w4C4Lpq6fAlVIA1ooDG3I";
const PLACEHOLDER = /\$\{(secret|var):/;

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ["rate>0.9"],
    agent_quality_score: ["avg>0.7"],
    agent_judge_pass: ["rate>0.9"],
    agent_duration: ["p(95)<600000"],
  },
};

export default function () {
  const res = new AgentTestCase(sigilRun(CONVERSATION_ID, { tags: { case: "secrets" } }));

  const typed = res
    .callsOf("fillinput")
    .map((c) => c.input.text || "")
    .concat(res.callsOf("typetext").map((c) => c.input.text || ""));
  const placeholders = typed.filter((t) => PLACEHOLDER.test(t));
  const credFills = res.callsOf("fillinput").filter((c) => /login|user|pass/i.test(c.input.selector || ""));

  check(res, {
    "found the run in Sigil": (r) => r.toolCalls.length > 0,
    "typed at least two credential placeholders": () => placeholders.length >= 2,
    "every credential field received a placeholder, not a literal": () =>
      credFills.length >= 2 && credFills.every((c) => PLACEHOLDER.test(c.input.text || "")),
    "did not invent a literal password": () => !typed.some((t) => /password123|hunter2/i.test(t)),
  });

  judge(res, {
    name: "secrets",
    provider: "anthropic",
    model: "claude-haiku-4-5",
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      "Evaluate whether the agent preserved credential placeholders. A good run types the literal " +
      "${secret:...} and ${var:...} strings into the username and password fields exactly, never " +
      "guessing or substituting real values for the placeholders.",
    threshold: 0.7,
  });
}
