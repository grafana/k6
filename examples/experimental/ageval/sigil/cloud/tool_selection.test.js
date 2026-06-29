// Evaluate a REAL browser agent's tool selection by reading its run from Sigil.
//
// Reads a recorded k6-abt run (a QuickPizza admin-login flow) from Sigil, wraps
// it in the native AgentTestCase, and asserts it used the right tools (fillinput
// for fields, clickelement for buttons) and reported each step — then scores the
// sequence with the LLM judge. Also covers step_reporting.
//
//   set -a; . ./.env; set +a            # ANTHROPIC_API_KEY, SIGIL_BASE, ABT_SIGIL_TOKEN
//   k6 run tool_selection.test.js
//   ABT_CONVERSATION_ID=<id> k6 run tool_selection.test.js   # point at another run
import { check } from "k6";
import { AgentTestCase, judge } from "k6/experimental/ageval";
import { sigilRun } from "../sigil.js";

const CONVERSATION_ID = __ENV.ABT_CONVERSATION_ID || "-gjXRUjZ58k-QLQn_QILK";

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ["rate>0.9"],
    agent_tool_correctness: ["rate>0.9"],
    agent_quality_score: ["avg>0.7"],
    agent_judge_pass: ["rate>0.9"],
    agent_duration: ["p(95)<600000"],
  },
};

export default function () {
  const res = new AgentTestCase(sigilRun(CONVERSATION_ID, { tags: { case: "tool_selection" } }));

  const reports = res.stepReports();
  check(res, {
    "found the run in Sigil": (r) => r.toolCalls.length > 0,
    "used fillinput for the inputs": (r) => r.callsOf("fillinput").length >= 1,
    "filled the username with admin": (r) =>
      r.callsOf("fillinput").some((c) => (c.input.text || "").includes("admin")),
    "used clickelement for the button": (r) => r.callsOf("clickelement").length >= 1,
    "did not typetext into form fields": (r) => r.callsOf("typetext").length === 0,
    "did not mouseclick a selectable button": (r) => r.callsOf("mouseclick").length === 0,
    "investigated the page first (screenshot or html)": (r) =>
      r.toolSequence().includes("screenshot") || r.toolSequence().includes("html"),
    // step_reporting: one report per step, 1-based, accurate flags.
    "reported at least one step": () => reports.length >= 1,
    "first report is step 1": () => reports.length > 0 && reports[0].input.step_index === 1,
    "step indices are 1-based and ascending": () => reports.every((r, i) => r.input.step_index === i + 1),
    "no failed steps on a successful login": () => res.failedSteps().length === 0,
  });

  res.expectSequence([{ name: "fillinput" }, { name: "clickelement" }], {
    mode: "in-order",
    allowOtherCalls: true,
  });

  judge(res, {
    name: "tool_selection",
    provider: "anthropic",
    model: "claude-haiku-4-5",
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      "Evaluate whether the agent tool sequence was logical and efficient for filling and submitting a " +
      "login form. A good sequence inspects the page (screenshot/html), uses fillinput (not typetext) for " +
      "the username and password inputs, uses clickelement (not mouseclick) for the submit button, and " +
      "calls report_step after each step with a truthful success flag.",
    threshold: 0.7,
  });
}
