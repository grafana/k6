# Evaluating real agent frameworks with `k6/experimental/ageval`

These examples validate that ageval is **framework-agnostic** and that its scoring
is **valid** (it discriminates good runs from bad, and reacts to real model
changes). Four very different agent frameworks are evaluated with the *same*
assertions and *zero* changes to the k6 module:

| Example | Framework | Mode |
|---|---|---|
| [`pydantic-ai/`](./pydantic-ai/) | [Pydantic-AI](https://ai.pydantic.dev) | **live** — one `k6 run` executes the agent |
| [`langgraph/`](./langgraph/) | [LangGraph](https://langchain-ai.github.io/langgraph/) ReAct agent | recorded |
| [`openai-agents/`](./openai-agents/) | [OpenAI Agents SDK](https://openai.github.io/openai-agents-python/) (on Claude via LiteLLM) | recorded |
| [`crewai/`](./crewai/) | [CrewAI](https://docs.crewai.com) — **multi-agent** crew | recorded |

## The one pattern: canonical shape

ageval's only hard contract is the canonical trajectory:

```json
{ "input": "...", "output": "...",
  "toolCalls": [{ "name": "...", "input": {}, "output": "..." }],
  "usage": { "inputTokens": 0, "outputTokens": 0 } }
```

Every framework exposes its run differently, so each example has a tiny
**capture shim** (~15-20 lines, `agent.py` / `capture.py`) that maps the
framework's result onto this shape:

- **Pydantic-AI** — `result.output`, `result.all_messages()` (`ToolCallPart`/`ToolReturnPart`), `result.usage`.
- **LangGraph** — message list: `AIMessage.tool_calls` + `ToolMessage` + `usage_metadata`.
- **OpenAI Agents SDK** — `result.final_output`, `result.new_items` (`ToolCallItem`/`ToolCallOutputItem`), `result.context_wrapper.usage`.
- **CrewAI** — a `TRACE` list inside the tools (captures calls across both agents), `CrewOutput.raw` + `token_usage`.

That shim is the *entire* integration cost. No new k6/Go code, no per-framework
adapter baked into the module. (For agents instrumented with OpenTelemetry GenAI /
OpenInference, a future built-in `otel` adapter would remove even the shim.)

**Live vs recorded.** The live example (`pydantic-ai`) runs the agent inside
`k6 run` via `CliAgent`. The recorded examples replay a committed
`trajectory.json` via `new AgentTestCase(...)`, so they are deterministic and CI-safe — only
the LLM judge needs a key. Each recorded dir's `capture.py` regenerates its fixture.

## What each track validates

- **Usefulness** — four real frameworks (including a multi-agent crew) plug in with only a shim.
- **Validity, tool dimension** — `expectSequence(...)` is a deterministic, non-LLM oracle: the agent must call the right tools, with the right args, in the right order (in CrewAI, *across* the Researcher → Writer handoff).
- **Validity, answer dimension** — the LLM `judge` scores the final answer against a rubric.
- **Discrimination** — `pydantic-ai/eval_golden.test.js` feeds a real good run and a tampered bad run (wrong invoice, leaked id) through the same assertions and checks that good passes and bad fails. (Verified: judge 1.0 vs 0.0.)

## Differential check: Sonnet → Haiku

The examples default to the cheaper `claude-haiku-4-5` as the **agent** model (the
judge stays on a stronger model for fair grading). Re-running the identical evals
on Haiku vs Claude Sonnet is itself a validation that the eval reacts to real model
changes:

- **Pydantic-AI (live):** still 4/4; cost dropped from **$0.0067 → $0.0023** (measured by `agent_cost_usd`).
- **LangGraph, OpenAI SDK:** unchanged (4/4).
- **CrewAI:** Haiku phrased the answer as *"widget manufacturer"* (singular) instead of *"widgets"*, which tripped an over-strict `/widgets/` string check **while the LLM judge still scored it 1.0**. Lesson the differential surfaced: pair a deterministic tool oracle with a semantic judge, and avoid brittle exact-string checks. (The check was loosened to `/widget/i`.)

Set `AGENT_MODEL=claude-sonnet-4-5` to regenerate fixtures on Sonnet and compare.

## Running everything

```bash
# build k6 once (from repo root)
go build -o k6 .

# recorded (deterministic; only the judge key needed)
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  ./k6 run examples/experimental/ageval/frameworks/langgraph/eval.test.js
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  ./k6 run examples/experimental/ageval/frameworks/openai-agents/eval.test.js
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  ./k6 run examples/experimental/ageval/frameworks/crewai/eval.test.js

# discrimination
ANTHROPIC_API_KEY_JUDGE=sk-ant-...  ./k6 run examples/experimental/ageval/frameworks/pydantic-ai/eval_golden.test.js

# live (also runs the agent; needs the agent key + a venv — see pydantic-ai/README.md)
cd examples/experimental/ageval/frameworks/pydantic-ai
ANTHROPIC_API_KEY_AGENT=sk-ant-... ANTHROPIC_API_KEY_JUDGE=sk-ant-... PYTHON=./venv/bin/python  ../../../../../k6 run eval.test.js
```

`ANTHROPIC_API_KEY_AGENT` is used by the agent; `ANTHROPIC_API_KEY_JUDGE` by the judge.
