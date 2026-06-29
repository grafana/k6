// Evaluate tool selection for a native <select> dropdown, read from Sigil.
//
// A good agent uses the `selectoption` tool for a <select> (not typetext or
// mouseclick) and picks the requested option.
//
//   set -a; . ./.env; set +a
//   k6 run tool_selection_dropdown.test.js
//
// Reproduce on your own stack: host a page with a native <select id="colors-options">
// and run a k6-abt test with the step: Select "Green" from the "Choose a color" dropdown.
import { check } from "k6";
import { AgentTestCase, judge } from "k6/experimental/ageval";
import { sigilRun } from "../sigil.js";

const CONVERSATION_ID = __ENV.ABT_CONVERSATION_ID || "bsg6ToMBBMT3y891Htz-Z";
const WANT_OPTION = __ENV.DROPDOWN_OPTION || "green";

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
  const res = new AgentTestCase(sigilRun(CONVERSATION_ID, { tags: { case: "dropdown" } }));

  const selects = res.callsOf("selectoption");
  const re = new RegExp(WANT_OPTION, "i");

  check(res, {
    "found the run in Sigil": (r) => r.toolCalls.length > 0,
    "used selectoption for the dropdown": () => selects.length >= 1,
    "did not typetext into the dropdown": (r) => r.callsOf("typetext").length === 0,
    "did not mouseclick the dropdown": (r) => r.callsOf("mouseclick").length === 0,
    "selected the requested option": () =>
      selects.some((c) => re.test(c.input.value || "") || re.test(c.input.label || "")),
  });

  judge(res, {
    name: "dropdown",
    provider: "anthropic",
    model: "claude-haiku-4-5",
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'Evaluate whether the agent correctly selected an option from a <select> dropdown. A good sequence ' +
      "uses selectoption (not mouseclick or typetext) targeting the \"Choose a color\" dropdown " +
      '(#colors-options) and selects Green by value ("green") or label ("Green"), then reports the step.',
    threshold: 0.7,
  });
}
