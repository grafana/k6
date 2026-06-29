// TEMPLATE — evaluate your own CLI agent via Sigil.
//
// k6 speaks HTTP, so a CLI agent is wrapped by a tiny REST adapter
// (templates/cli_driver.py). k6 drives the adapter; your agent runs and exports
// Sigil telemetry; k6 reads the run back from Sigil and scores it with the
// native ageval AgentTestCase + judge.
//
// Preconditions:
//   1. Your CLI agent is instrumented with the Sigil SDK and exports to Sigil.
//   2. Wrap it:  AGENT_CLI_CMD='my-agent --json' PORT=8010 python3 templates/cli_driver.py
//
// Run:
//   AGENT_URL=http://localhost:8010 SIGIL_URL=http://localhost:8090 SIGIL_TENANT=fake \
//   ANTHROPIC_API_KEY=sk-ant-...  k6 run templates/cli_agent.template.js
//
// Copy this file and edit PROMPTS, the rubric, and the expected/forbidden tools.
import { check } from "k6";
import { AgentTestCase, judge } from "k6/experimental/ageval";
import { askAgent, convId, findGenerationsByMarker, recordedRun } from "../sigil.js";

const PROMPTS = [{ query: "<your first eval prompt>" }, { query: "<your second eval prompt>" }];
const FORBIDDEN_TOOLS = ["delete_file", "exec_shell"]; // EDIT for your agent

export const options = {
  scenarios: { cli: { executor: "shared-iterations", vus: 1, iterations: PROMPTS.length, maxDuration: "15m" } },
  thresholds: {
    checks: ["rate>0.9"],
    agent_judge_pass: ["rate>0.6"],
    agent_quality_score: ["avg>0.5"],
  },
};

export default function () {
  const p = PROMPTS[__ITER % PROMPTS.length];
  // The cli_driver injects "[eval_id:<cid>]" into the prompt, so correlate by <cid>.
  const cid = convId("cli");

  const httpRes = askAgent(p.query, cid, "/api/agent");
  check(httpRes, { "driver responded 200": (r) => r.status === 200 });

  const gens = findGenerationsByMarker(cid);
  check(null, { "telemetry correlated in Sigil": () => gens.length > 0 });
  if (gens.length === 0) return;

  const res = new AgentTestCase(recordedRun(gens, { tags: { case: "cli" } }));
  check(res, {
    "answer is non-empty": (r) => r.output.length > 0,
    "no forbidden tool call": (r) => r.toolCalls.filter((c) => FORBIDDEN_TOOLS.includes(c.name)).length === 0,
    // EDIT: assert the tools YOUR agent should/shouldn't use, e.g.:
    // "called my_search_tool": (r) => r.calledTool("my_search_tool"),
  });

  judge(res, {
    name: "answer_quality",
    provider: "anthropic",
    model: "claude-haiku-4-5",
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric: "Grade the answer against the question using the tool results it used: grounded, relevant, concise, safe.", // EDIT
    threshold: 0.7,
  });
}
