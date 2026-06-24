# Pydantic-AI × ageval (live, single `k6 run`)

Evaluates a **real [Pydantic-AI](https://ai.pydantic.dev) agent** end-to-end in one
`k6 run`. `agent.py` is a small bank-support agent (two tools: `get_customer`,
`get_invoice`) running on Claude; on each run it prints a **canonical ageval
trajectory** as JSON. k6's `ExternalAgent` runs it and grades the result.

This is the "framework app" pattern: ageval's only hard contract is the canonical
shape `{ output, toolCalls:[{name,input,output}], usage }`. A ~15-line shim in
`agent.py` (`to_trajectory`) maps Pydantic-AI's `result.output` /
`result.all_messages()` (`ToolCallPart`/`ToolReturnPart`) / `result.usage` onto it.
No new k6/Go code is required.

## Setup

```bash
python3 -m venv venv
venv/bin/pip install pydantic-ai
```

## Run (live)

```bash
cd examples/experimental/ageval/frameworks/pydantic-ai
ANTHROPIC_API_KEY_AGENT=sk-ant-...  ANTHROPIC_API_KEY_JUDGE=sk-ant-... \
PYTHON=./venv/bin/python  k6 run eval.test.js
```

`ANTHROPIC_API_KEY_AGENT` is used by the agent; `ANTHROPIC_API_KEY_JUDGE` by the
LLM judge. `PYTHON` points `ExternalAgent` at the venv interpreter (defaults to
`python3`). The agent defaults to the cheap `claude-haiku-4-5`
(set `AGENT_MODEL=claude-sonnet-4-5` to compare). Verified result: 4/4 checks,
`agent_tool_correctness` 100%, `agent_quality_score` 1.0, cost ~$0.002/run on Haiku
(~$0.007 on Sonnet) — see the differential note in [`../README.md`](../README.md).

## What it validates

- **Usefulness** — a real third-party framework agent plugs in with only the shim.
- **Validity (tool dimension)** — `expectSequence([get_customer, get_invoice{INV-123}])`
  is a deterministic, non-LLM oracle: the agent must look the customer up *before*
  the invoice and query the right id.
- **Validity (answer dimension)** — the LLM `judge` scores the final answer.

## Golden good-vs-bad (`eval_golden.test.js`)

Proves the eval *discriminates* rather than rubber-stamps. Feeds two committed
fixtures of the same task — `trajectory.good.json` (real run) and
`trajectory.bad.json` (tampered: skips `get_customer`, wrong invoice, leaks the
internal `cust_123` id, uncertain answer) — through the same assertions and checks
that the good run passes and the bad run fails (deterministically, plus the judge).

```bash
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  k6 run eval_golden.test.js   # 8/8 checks pass
```

Regenerate `trajectory.good.json`:
`ANTHROPIC_API_KEY_AGENT=… ./venv/bin/python agent.py > trajectory.good.json`
