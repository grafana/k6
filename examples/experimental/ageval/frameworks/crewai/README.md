# CrewAI × ageval (recorded, multi-agent)

Evaluates a [CrewAI](https://docs.crewai.com) **multi-agent crew** with
`k6/experimental/ageval`: a Researcher (tool: `lookup_company`) hands off to a
Writer (tool: `publish_summary`). The captured `trajectory.json` spans **both
agents' tool calls**, so the eval grades the cross-agent pipeline — `expectSequence`
requires research *before* publishing. Replayed by `eval.test.js` (deterministic /
CI-safe; only the judge key is needed). See [`../README.md`](../README.md) for the
overall pattern.

## Run (replay the committed fixture)

```bash
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  k6 run eval.test.js
```

## Regenerate the fixture (live)

```bash
python3 -m venv venv && venv/bin/pip install "crewai[anthropic]"
# CrewAI is chatty, so capture.py writes the file directly (not stdout):
ANTHROPIC_API_KEY_AGENT=sk-ant-...  venv/bin/python capture.py        # writes ./trajectory.json
```

Defaults to `claude-haiku-4-5`; set `AGENT_MODEL=claude-sonnet-4-5` to compare models.
