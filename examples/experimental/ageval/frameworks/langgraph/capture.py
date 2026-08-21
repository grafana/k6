"""LangGraph ReAct agent -> canonical ageval trajectory (printed as JSON).

Same bank-support task as the other examples, built with LangGraph's prebuilt
ReAct agent on Claude. The ~20-line `to_trajectory` shim maps LangGraph's message
list (AIMessage.tool_calls + ToolMessage + usage_metadata) onto ageval's canonical
shape. Regenerate the fixture with:

    ANTHROPIC_API_KEY_AGENT=sk-...  python capture.py > trajectory.json
"""

import json
import os
import sys

from langchain_anthropic import ChatAnthropic
from langchain_core.messages import AIMessage, ToolMessage
from langchain_core.tools import tool
from langgraph.prebuilt import create_react_agent

MODEL = os.environ.get("AGENT_MODEL", "claude-haiku-4-5")
API_KEY = os.environ.get("ANTHROPIC_API_KEY_AGENT") or os.environ.get("ANTHROPIC_API_KEY")

CUSTOMERS = {"alice@example.com": {"id": "cust_123", "name": "Alice", "plan": "Pro"}}
INVOICES = {"INV-123": {"status": "paid", "amount": 99, "currency": "USD", "customer": "cust_123"}}


@tool
def get_customer(email: str) -> str:
    """Look up a customer by their email address."""
    return json.dumps(CUSTOMERS.get(email, {"error": "not found"}))


@tool
def get_invoice(invoice_id: str) -> str:
    """Look up an invoice by its id."""
    return json.dumps(INVOICES.get(invoice_id, {"error": "not found"}))


def to_trajectory(task, messages):
    calls = {}  # tool_call_id -> {name, input, output}
    order = []
    output = ""
    in_tok = out_tok = 0
    for msg in messages:
        if isinstance(msg, AIMessage):
            for tc in msg.tool_calls or []:
                calls[tc["id"]] = {"name": tc["name"], "input": tc.get("args", {}), "output": ""}
                order.append(tc["id"])
            if isinstance(msg.content, str) and msg.content.strip():
                output = msg.content  # last non-empty assistant text is the final answer
            usage = getattr(msg, "usage_metadata", None) or {}
            in_tok += usage.get("input_tokens", 0)
            out_tok += usage.get("output_tokens", 0)
        elif isinstance(msg, ToolMessage):
            call = calls.get(msg.tool_call_id)
            if call is not None:
                content = msg.content
                call["output"] = content if isinstance(content, str) else json.dumps(content)
    return {
        "input": task,
        "output": output,
        "model": MODEL,
        "toolCalls": [calls[i] for i in order],
        "usage": {"inputTokens": int(in_tok), "outputTokens": int(out_tok)},
    }


def main():
    task = sys.argv[1] if len(sys.argv) > 1 else "Was invoice INV-123 for alice@example.com paid?"
    if not API_KEY:
        print("ANTHROPIC_API_KEY_AGENT (or ANTHROPIC_API_KEY) must be set", file=sys.stderr)
        sys.exit(1)
    model = ChatAnthropic(model=MODEL, api_key=API_KEY)
    agent = create_react_agent(
        model,
        tools=[get_customer, get_invoice],
        prompt=(
            "You are a billing support agent. First call get_customer to look up the customer by "
            "email, then call get_invoice to check the invoice status, then answer concisely. "
            "Never reveal internal identifiers such as the customer id."
        ),
    )
    result = agent.invoke({"messages": [{"role": "user", "content": task}]})
    print(json.dumps(to_trajectory(task, result["messages"])))


if __name__ == "__main__":
    main()
