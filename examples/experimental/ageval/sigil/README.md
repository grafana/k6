# Sigil source for `k6/experimental/ageval`

Evaluate **real** agent runs by reading their [Sigil](https://grafana.com/) (AI
observability) telemetry and scoring them with the native ageval module.

`sigil.js` is a thin *source*: it fetches a conversation's generations from Sigil
and maps them to the canonical recorded-run shape
(`{ input, output, toolCalls: [{ name, input, output }], model, usage, durationMs }`).
You then evaluate it exactly like [`../real_agent.js`](../real_agent.js):

```js
import { AgentTestCase, judge } from 'k6/experimental/ageval';
import { sigilRun } from '../sigil.js';

const res = new AgentTestCase(sigilRun(conversationId));
check(res, { 'used the right tool': (r) => r.calledTool('selectoption') });
res.expectSequence([{ name: 'fillinput' }, { name: 'clickelement' }]);
judge(res, { name: 'quality', provider: 'anthropic', model: 'claude-haiku-4-5',
             apiKey: __ENV.ANTHROPIC_API_KEY, rubric: '...', threshold: 0.7 });
```

All assertion/scoring (and the `agent_*` metrics) come from the native module —
`sigil.js` only supplies the trajectory. It complements `AgentSimulator` (mocked
tools), `CliAgent` (parse stdout), and the cc/codex adapters (parse a trace): the
source here is **live production telemetry**.

## Layout

| Path | Contains |
|------|----------|
| `sigil.js` | The source: Sigil readers (cloud + local), tool-arg decoding, and `recordedRun()` / `sigilRun()` mappers to the ageval recorded-run shape. Also `askAgent()`/`convId()` for driving local agents. |
| `cloud/` | Read **real** runs from a cloud Sigil app-plugin proxy and evaluate them: `tool_selection`, `tool_selection_dropdown`, `secrets`, `strict_failure`, `prompt_injection`. |
| `local/` | Drive a local agent (REST / chat UI) and read its run from a local Sigil: `agent_eval.js`, `security_redteam.js`, `agent_eval_browser.js`. |
| `templates/` | Copy-and-edit starters for your own agent (`cli_agent.template.js`, `web_gui_agent.template.js`) + `cli_driver.py` (a CLI→HTTP adapter). |

## Prerequisites

- A k6 build that includes `k6/experimental/ageval`.
- `ANTHROPIC_API_KEY` — the judge is Anthropic-only.
- Secrets in `.env` (gitignored): `set -a; . ./.env; set +a`.

| Var | Used by | Meaning |
|-----|---------|---------|
| `ANTHROPIC_API_KEY` | all | LLM-as-judge key (`ANTHROPIC_API_KEY_JUDGE` also honored). |
| `SIGIL_BASE` | cloud | `https://<your-stack>/api/plugins/grafana-sigil-app/resources/query` (no default). |
| `ABT_SIGIL_TOKEN` | cloud | Grafana service-account token with the **Sigil Reader** role. |
| `ABT_CONVERSATION_ID` | cloud | Override the conversation a cloud example reads. |
| `AGENT_URL` / `SIGIL_URL` / `SIGIL_TENANT` | local | Local agent endpoint + local Sigil (defaults `:8000` / `:8090` / `fake`). |

## Run

```sh
set -a; . ./.env; set +a
k6 run cloud/tool_selection.test.js     # cloud: read a real run from Sigil + judge
k6 run local/agent_eval.js              # local: drive an agent, read its run, judge
```

The cloud examples default to example conversation IDs from the stack used to
validate them; set `SIGIL_BASE` + `ABT_SIGIL_TOKEN` for that stack (shared
separately), or override `ABT_CONVERSATION_ID` to point at a run on your own
stack. No stack URL is committed here.

## Evaluate your own agent

1. Instrument your agent with the Sigil SDK so each LLM call is exported as a
   generation (request, response, tools, tool calls + args, tokens).
2. Point the source at your Sigil — cloud: `SIGIL_BASE` + `ABT_SIGIL_TOKEN`;
   local: `SIGIL_URL` (+ `SIGIL_TENANT`).
3. Build a recorded run and evaluate it: `new AgentTestCase(sigilRun(id))` (cloud)
   or `new AgentTestCase(recordedRun(fetchGenerations(cid)))` (local), then
   `check` / `expectSequence` / `judge`. See `templates/`.
