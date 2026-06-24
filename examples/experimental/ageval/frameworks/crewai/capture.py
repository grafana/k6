"""CrewAI MULTI-AGENT crew -> canonical ageval trajectory (printed as JSON).

A two-agent crew on Claude: a Researcher (tool: lookup_company) hands off to a
Writer (tool: publish_summary). This exercises ageval's multi-agent / sub-agent
story: the trajectory spans BOTH agents' tool calls, and the eval grades the
combined sequence. Tool calls are captured by a small TRACE list inside the tools
(version-independent). CrewAI is chatty and some of its loggers bypass a stdout
redirect, so this writes the JSON straight to a file (default ./trajectory.json)
rather than stdout. Regenerate with:

    ANTHROPIC_API_KEY_AGENT=sk-...  python capture.py [out.json]
"""

import contextlib
import json
import os
import sys

os.environ.setdefault("CREWAI_DISABLE_TELEMETRY", "true")

from crewai import LLM, Agent, Crew, Process, Task  # noqa: E402
from crewai.tools import tool  # noqa: E402

MODEL = os.environ.get("AGENT_MODEL", "claude-haiku-4-5")
API_KEY = os.environ.get("ANTHROPIC_API_KEY_AGENT") or os.environ.get("ANTHROPIC_API_KEY")

FACTS = {"Acme Corp": {"founded": 1998, "industry": "widgets", "hq": "Springfield"}}
TRACE = []  # ordered tool calls across both agents


@tool("lookup_company")
def lookup_company(name: str) -> str:
    """Look up basic facts (founding year, industry, HQ) about a company by name."""
    out = json.dumps(FACTS.get(name, {"error": "unknown company"}))
    TRACE.append({"name": "lookup_company", "input": {"name": name}, "output": out})
    return out


@tool("publish_summary")
def publish_summary(summary: str) -> str:
    """Publish the final one-sentence company summary."""
    TRACE.append({"name": "publish_summary", "input": {"summary": summary}, "output": "published"})
    return "published"


def main():
    task_text = sys.argv[1] if len(sys.argv) > 1 else "Acme Corp"
    if not API_KEY:
        print("ANTHROPIC_API_KEY_AGENT (or ANTHROPIC_API_KEY) must be set", file=sys.stderr)
        sys.exit(1)

    llm = LLM(model=f"anthropic/{MODEL}", api_key=API_KEY)

    researcher = Agent(
        role="Company Researcher",
        goal="Find accurate facts about a company using the lookup_company tool.",
        backstory="You look up authoritative facts and never invent them.",
        tools=[lookup_company],
        llm=llm,
        verbose=False,
    )
    writer = Agent(
        role="Summary Writer",
        goal="Write a concise one-sentence company summary and publish it.",
        backstory="You turn researched facts into a single clear sentence.",
        tools=[publish_summary],
        llm=llm,
        verbose=False,
    )

    research = Task(
        description=f"Look up the company '{task_text}' and report its founding year and industry.",
        expected_output="The company's founding year and industry.",
        agent=researcher,
    )
    write = Task(
        description="Write a one-sentence summary from the research, then publish it with publish_summary.",
        expected_output="A one-sentence published summary.",
        agent=writer,
        context=[research],
    )

    crew = Crew(agents=[researcher, writer], tasks=[research, write], process=Process.sequential, verbose=False)

    # Keep stdout clean for the JSON; send any crew/LLM chatter to stderr.
    with contextlib.redirect_stdout(sys.stderr):
        result = crew.kickoff()

    usage = getattr(result, "token_usage", None)
    in_tok = int(getattr(usage, "prompt_tokens", 0) or 0)
    out_tok = int(getattr(usage, "completion_tokens", 0) or 0)
    trajectory = {
        "input": task_text,
        "output": str(getattr(result, "raw", result)),
        "model": MODEL,
        "toolCalls": TRACE,
        "usage": {"inputTokens": in_tok, "outputTokens": out_tok},
    }
    out_path = sys.argv[2] if len(sys.argv) > 2 else "trajectory.json"
    with open(out_path, "w") as f:
        json.dump(trajectory, f, indent=4)
    print(f"wrote {out_path}", file=sys.stderr)


if __name__ == "__main__":
    main()
