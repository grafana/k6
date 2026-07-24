# OpenAI Agents SDK × ageval (recorded)

Evaluates an [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/)
agent (bank-support task) with `k6/experimental/ageval`. It runs on **Claude via
the SDK's LiteLLM extension** — no OpenAI key needed. The run is captured once into
`trajectory.json` and replayed by `eval.test.js` (deterministic / CI-safe; only the
judge key is needed). See [`../README.md`](../README.md) for the overall pattern.

## Run (replay the committed fixture)

```bash
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  k6 run eval.test.js
```

## Regenerate the fixture (live)

```bash
python3 -m venv venv && venv/bin/pip install "openai-agents[litellm]"
ANTHROPIC_API_KEY_AGENT=sk-ant-...  venv/bin/python capture.py > trajectory.json
```

Defaults to `claude-haiku-4-5`; set `AGENT_MODEL=claude-sonnet-4-5` to compare models.
