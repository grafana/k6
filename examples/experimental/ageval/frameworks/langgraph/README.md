# LangGraph × ageval (recorded)

Evaluates a [LangGraph](https://langchain-ai.github.io/langgraph/) ReAct agent
(bank-support task) with `k6/experimental/ageval`. The run is captured once into
`trajectory.json` (canonical ageval shape) and replayed by `eval.test.js`, so the
test is deterministic and CI-safe — only the judge key is needed. See
[`../README.md`](../README.md) for the overall pattern.

## Run (replay the committed fixture)

```bash
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  k6 run eval.test.js
```

## Regenerate the fixture (live)

```bash
python3 -m venv venv && venv/bin/pip install langgraph langchain-anthropic
ANTHROPIC_API_KEY_AGENT=sk-ant-...  venv/bin/python capture.py > trajectory.json
```

Defaults to `claude-haiku-4-5`; set `AGENT_MODEL=claude-sonnet-4-5` to compare models.
