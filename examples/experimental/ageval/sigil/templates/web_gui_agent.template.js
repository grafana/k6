// TEMPLATE — evaluate your own web-GUI / chatbox agent via Sigil (no Playwright).
//
// k6's built-in browser module drives the chat UI directly: type the prompt, read
// the rendered answer. The agent behind the UI exports Sigil telemetry; k6 reads
// the run back from Sigil and scores it with AgentTestCase + judge.
//
// Preconditions:
//   1. The agent behind the UI is instrumented with the Sigil SDK and exports to Sigil.
//   2. Chrome/Chromium available (set K6_BROWSER_EXECUTABLE_PATH if needed).
//
// Run:
//   CHAT_URL=http://localhost:8000/ SIGIL_URL=http://localhost:8090 SIGIL_TENANT=fake \
//   ANTHROPIC_API_KEY=sk-ant-...  k6 run templates/web_gui_agent.template.js
//
// Copy this file and edit CHAT_URL, the selectors, PROMPTS, and the rubric.
import { browser } from "k6/browser";
import { check, sleep } from "k6";
import { AgentTestCase, judge } from "k6/experimental/ageval";
import { findGenerationsByMarker, recordedRun } from "../sigil.js";

const CHAT_URL = __ENV.CHAT_URL || "http://localhost:8000/";
const PROMPT_SELECTOR = __ENV.PROMPT_SELECTOR || "#query"; // input box
const SUBMIT_SELECTOR = __ENV.SUBMIT_SELECTOR || "#send-btn"; // send button ("" => press Enter)
const RESPONSE_SELECTOR = __ENV.RESPONSE_SELECTOR || ".message.assistant"; // assistant bubbles

const PROMPTS = [{ query: "<your first eval prompt>" }, { query: "<your second eval prompt>" }];

export const options = {
  scenarios: {
    ui: {
      executor: "shared-iterations",
      vus: 1,
      iterations: PROMPTS.length,
      maxDuration: "15m",
      options: { browser: { type: "chromium" } },
    },
  },
  thresholds: {
    checks: ["rate>0.8"],
    agent_judge_pass: ["rate>0.6"],
    agent_quality_score: ["avg>0.5"],
  },
};

export default async function () {
  const p = PROMPTS[__ITER % PROMPTS.length];
  // Chat UIs rarely let you set a conversation_id, so embed a unique marker in the
  // prompt and correlate the run to its Sigil telemetry by that marker.
  const marker = `eval-${__VU}-${__ITER}-${Date.now()}`;
  const prompt = `[${marker}] ${p.query}`;

  const page = await browser.newPage();
  let answerText = "";
  try {
    await page.goto(CHAT_URL, { waitUntil: "load" });
    const before = (await page.$$(RESPONSE_SELECTOR)).length;
    await page.locator(PROMPT_SELECTOR).fill(prompt);
    if (SUBMIT_SELECTOR) await page.locator(SUBMIT_SELECTOR).click();
    else await page.locator(PROMPT_SELECTOR).press("Enter");
    for (let i = 0; i < 180; i++) {
      const els = await page.$$(RESPONSE_SELECTOR);
      if (els.length > before) { answerText = (await els[els.length - 1].textContent()) || ""; break; }
      sleep(1);
    }
  } finally {
    await page.close();
  }
  check(answerText, { "UI rendered a response": (t) => t.length > 0 });

  const gens = findGenerationsByMarker(marker);
  check(null, { "telemetry correlated in Sigil": () => gens.length > 0 });
  if (gens.length === 0) return;

  const res = new AgentTestCase(recordedRun(gens, { tags: { case: "web_ui" } }));

  judge(res, {
    name: "answer_quality",
    provider: "anthropic",
    model: "claude-haiku-4-5",
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric: "Grade the answer against the question: grounded, relevant, concise, safe.", // EDIT
    threshold: 0.7,
  });
}
