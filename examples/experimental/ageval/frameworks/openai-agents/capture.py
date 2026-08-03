"""OpenAI Agents SDK agent -> canonical ageval trajectory (printed as JSON).

Same bank-support task as the other examples. The OpenAI Agents SDK is run on
Claude via its LiteLLM extension (no OpenAI key needed). The `to_trajectory` shim
maps result.final_output / result.new_items (ToolCallItem / ToolCallOutputItem) /
result.context_wrapper.usage onto ageval's canonical shape. Regenerate with:

    ANTHROPIC_API_KEY_AGENT=sk-...  python capture.py > trajectory.json
"""

import json
import os
import sys

from agents import Agent, Runner, function_tool, set_tracing_disabled
from agents.extensions.models.litellm_model import LitellmModel
from agents.items import ToolCallItem, ToolCallOutputItem

# We run on Claude via LiteLLM; disable the SDK's default trace export to OpenAI
# (which would otherwise warn about a missing OpenAI key).
set_tracing_disabled(True)

MODEL = os.environ.get("AGENT_MODEL", "claude-haiku-4-5")
API_KEY = os.environ.get("ANTHROPIC_API_KEY_AGENT") or os.environ.get("ANTHROPIC_API_KEY")

CUSTOMERS = {"alice@example.com": {"id": "cust_123", "name": "Alice", "plan": "Pro"}}
INVOICES = {"INV-123": {"status": "paid", "amount": 99, "currency": "USD", "customer": "cust_123"}}


@function_tool
def get_customer(email: str) -> str:
    """Look up a customer by their email address."""
    return json.dumps(CUSTOMERS.get(email, {"error": "not found"}))


@function_tool
def get_invoice(invoice_id: str) -> str:
    """Look up an invoice by its id."""
    return json.dumps(INVOICES.get(invoice_id, {"error": "not found"}))


def _args_of(item):
    raw = getattr(item, "raw_item", None)
    raw_args = getattr(raw, "arguments", None)
    if isinstance(raw_args, str):
        try:
            return json.loads(raw_args or "{}")
        except json.JSONDecodeError:
            return {}
    return raw_args or {}


def _output_of(item):
    out = getattr(item, "output", None)
    if out is None:
        raw = getattr(item, "raw_item", None)
        out = raw.get("output") if isinstance(raw, dict) else raw
    return out if isinstance(out, str) else json.dumps(out)


def to_trajectory(task, result):
    calls = {}  # call_id -> {name, input, output}
    order = []
    for item in result.new_items:
        if isinstance(item, ToolCallItem):
            raw = getattr(item, "raw_item", None)
            name = getattr(item, "tool_name", None) or getattr(raw, "name", "")
            cid = getattr(item, "call_id", None) or getattr(raw, "call_id", "")
            calls[cid] = {"name": name, "input": _args_of(item), "output": ""}
            order.append(cid)
        elif isinstance(item, ToolCallOutputItem):
            cid = getattr(item, "call_id", "")
            if cid in calls:
                calls[cid]["output"] = _output_of(item)
    usage = result.context_wrapper.usage
    return {
        "input": task,
        "output": str(result.final_output),
        "model": MODEL,
        "toolCalls": [calls[i] for i in order],
        "usage": {
            "inputTokens": int(getattr(usage, "input_tokens", 0)),
            "outputTokens": int(getattr(usage, "output_tokens", 0)),
        },
    }


def main():
    task = sys.argv[1] if len(sys.argv) > 1 else "Was invoice INV-123 for alice@example.com paid?"
    if not API_KEY:
        print("ANTHROPIC_API_KEY_AGENT (or ANTHROPIC_API_KEY) must be set", file=sys.stderr)
        sys.exit(1)
    agent = Agent(
        name="billing-support",
        instructions=(
            "You are a billing support agent. First call get_customer to look up the customer by "
            "email, then call get_invoice to check the invoice status, then answer concisely. "
            "Never reveal internal identifiers such as the customer id."
        ),
        tools=[get_customer, get_invoice],
        model=LitellmModel(model=f"anthropic/{MODEL}", api_key=API_KEY),
    )
    result = Runner.run_sync(agent, task)
    print(json.dumps(to_trajectory(task, result)))


if __name__ == "__main__":
    main()
