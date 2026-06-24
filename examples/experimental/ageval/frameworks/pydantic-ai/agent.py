"""Pydantic-AI bank-support agent that emits a canonical ageval trajectory.

This is the ~15-line "capture shim" pattern: run a real framework agent, then map
its result onto ageval's only hard contract — the canonical trajectory shape
`{input, output, toolCalls:[{name,input,output}], usage}` — and print it as JSON.
k6's ExternalAgent runs this and grades the result, all in one `k6 run`.

    ANTHROPIC_API_KEY_AGENT=sk-...  python agent.py "Was invoice INV-123 for alice@example.com paid?"
"""

import json
import os
import sys

from pydantic_ai import Agent
from pydantic_ai.messages import ToolCallPart, ToolReturnPart
from pydantic_ai.models.anthropic import AnthropicModel
from pydantic_ai.providers.anthropic import AnthropicProvider

MODEL = os.environ.get("AGENT_MODEL", "claude-haiku-4-5")
API_KEY = os.environ.get("ANTHROPIC_API_KEY_AGENT") or os.environ.get("ANTHROPIC_API_KEY")

# Deterministic mock backend so the eval is reproducible.
CUSTOMERS = {"alice@example.com": {"id": "cust_123", "name": "Alice", "plan": "Pro"}}
INVOICES = {"INV-123": {"status": "paid", "amount": 99, "currency": "USD", "customer": "cust_123"}}

agent = Agent(
    AnthropicModel(MODEL, provider=AnthropicProvider(api_key=API_KEY)) if API_KEY else MODEL,
    system_prompt=(
        "You are a billing support agent. First call get_customer to look up the customer by "
        "email, then call get_invoice to check the invoice status, then answer concisely. "
        "Never reveal internal identifiers such as the customer id."
    ),
)


@agent.tool_plain
def get_customer(email: str) -> str:
    """Look up a customer by their email address."""
    return json.dumps(CUSTOMERS.get(email, {"error": "not found"}))


@agent.tool_plain
def get_invoice(invoice_id: str) -> str:
    """Look up an invoice by its id."""
    return json.dumps(INVOICES.get(invoice_id, {"error": "not found"}))


def to_trajectory(task, result):
    """Map a Pydantic-AI run onto the canonical ageval trajectory shape."""
    calls = {}  # tool_call_id -> {name, input, output}
    order = []
    for msg in result.all_messages():
        for part in getattr(msg, "parts", []):
            if isinstance(part, ToolCallPart):
                try:
                    args = part.args_as_dict()
                except Exception:
                    args = {}
                calls[part.tool_call_id] = {"name": part.tool_name, "input": args, "output": ""}
                order.append(part.tool_call_id)
            elif isinstance(part, ToolReturnPart):
                call = calls.get(part.tool_call_id)
                if call is not None:
                    content = part.content
                    call["output"] = content if isinstance(content, str) else json.dumps(content)
    # result.usage is an attribute in pydantic-ai 2.x; older versions exposed a method.
    usage = result.usage() if callable(getattr(result, "usage", None)) else result.usage
    in_tok = getattr(usage, "input_tokens", None) or getattr(usage, "request_tokens", 0) or 0
    out_tok = getattr(usage, "output_tokens", None) or getattr(usage, "response_tokens", 0) or 0
    return {
        "input": task,
        "output": str(result.output),
        "model": MODEL,
        "toolCalls": [calls[i] for i in order],
        "usage": {"inputTokens": int(in_tok), "outputTokens": int(out_tok)},
    }


def main():
    task = sys.argv[1] if len(sys.argv) > 1 else "Was invoice INV-123 for alice@example.com paid?"
    if not API_KEY:
        print("ANTHROPIC_API_KEY_AGENT (or ANTHROPIC_API_KEY) must be set", file=sys.stderr)
        sys.exit(1)
    result = agent.run_sync(task)
    # Only the canonical JSON goes to stdout; ExternalAgent parses it with JSON.parse.
    print(json.dumps(to_trajectory(task, result)))


if __name__ == "__main__":
    main()
