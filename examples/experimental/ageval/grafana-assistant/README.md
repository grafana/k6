# Evaluating the Grafana Assistant (k6 authoring) over its REST API with k6 experimental module ageval

This evaluates the **real Grafana Assistant** — its "k6 Script Authoring" behavior —
by calling its local **A2A REST endpoint** from k6 and grading the result with
`k6/experimental/ageval`. One `k6 run`: k6 POSTs the A2A `message/stream` request,
parses the SSE trajectory (`step.toolCall` / `step.message` / `step.complete`) into
`{output, toolCalls, usage}`, wraps it in `new AgentTestCase(...)`, and grades with
`check` + an LLM-as-judge — emitting the standard `agent_*` metrics (and you can
run it under VUs to measure quality + latency/cost under load).

## Why the llmspec env (not `mise run up`)

The default `mise run up` stack runs production-shaped auth (`ASSISTANT_USE_AUTH_API=true`,
needs a cloud **ID token**). The **llmspec env** runs local auth and seeds
everything, so a service-account token works:

```bash
# from grafana-assistant-app/
mise run llmspec:env:up        # assistant A2A on http://localhost:9191
```

## Auth recipe (the gotchas)

The A2A chat endpoint has three gates; the script + these steps clear all three:

1. **Terms** — `setup()` PUTs `/api/v1/settings/accept-terms`.
2. **Agent-task auth** — the agent needs a forwardable **Grafana service-account
   token**, sent as **both** `Authorization: Bearer <glsa>` **and** `X-Grafana-API-Key`
   (basic `admin:admin` is *not* enough — it gives "no authentication credentials").
   Mint one against the local Grafana (`:3300`, admin:admin):

   ```bash
   G=http://localhost:3300
   SAID=$(curl -s -u admin:admin -X POST $G/api/serviceaccounts \
     -H 'Content-Type: application/json' -d '{"name":"ageval-k6","role":"Admin"}' \
     | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
   GLSA=$(curl -s -u admin:admin -X POST $G/api/serviceaccounts/$SAID/tokens \
     -H 'Content-Type: application/json' -d '{"name":"ageval-k6-tok"}' \
     | sed -n 's/.*"key":"\([^"]*\)".*/\1/p')
   ```
3. **LLM creds** — the env's `.env` must have the model provider creds (the same
   ones llmspec uses).

## Run

```bash
ANTHROPIC_API_KEY_JUDGE=sk-ant-... \
A2A_URL=http://localhost:9191/api/v1/a2a \
GRAFANA_TOKEN="$GLSA" ORG_ID=1 \
  k6 run examples/experimental/ageval/grafana-assistant/k6_authoring.test.js
```

`GRAFANA_TOKEN` may be a `glsa_…` service-account token (sent as Bearer +
`X-Grafana-API-Key`) or `basic:user:pass` (HTTP Basic). For the agent task it must
be a service-account token.

## Result (verified)

A real run produced a valid k6 script and graded green:

```
✓ produced a response
✓ authored a k6 script (tool or text)
✓ script imports k6/http
✓ includes thresholds
agent_quality_score: avg=1      agent_judge_pass: 100%
```

## Caveat: `k6_script_handler` is a frontend tool

The assistant's k6-authoring tool (`k6_script_handler`) executes in the **browser**.
Over pure backend REST it isn't in the tool set, so the assistant returns the script
**in its message text** (which this eval grades). The script accepts either the
tool call *or* an inline script. To exercise the real `k6_script_handler` tool call,
use the frontend / `--browser-tools` runtime (as llmspec's strict k6 scenario does).
