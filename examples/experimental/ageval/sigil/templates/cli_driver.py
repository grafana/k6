#!/usr/bin/env python3
"""HTTP wrapper for CLI-only agents.

k6 speaks HTTP. If the agent under test is a CLI, run this adapter locally and
point k6's AGENT_URL at it. The adapter invokes the CLI with the prompt either
on stdin or as argv.

Required env:
  AGENT_CLI_CMD        command to run (e.g. "my-agent --json")
Optional env:
  AGENT_CLI_MODE       stdin | argv  (default stdin)
  PORT                 default 8010

API:
  POST /api/agent {"query":"...","conversation_id":"..."}
  -> {"answer":"stdout", "conversation_id":"...", "blocked":false}

Correlation trick: if the CLI cannot accept a conversation id, include the id in
the prompt text itself. Sigil/k6 can then search captured conversations for it.
"""

from __future__ import annotations

import json
import os
import shlex
import subprocess
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


CMD = os.environ.get("AGENT_CLI_CMD", "").strip()
MODE = os.environ.get("AGENT_CLI_MODE", "stdin").strip().lower()
PORT = int(os.environ.get("PORT", "8010"))


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def _send(self, status: int, body: dict) -> None:
        raw = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_POST(self):
        if self.path != "/api/agent":
            self._send(404, {"error": "not found"})
            return
        if not CMD:
            self._send(500, {"error": "AGENT_CLI_CMD is required"})
            return
        length = int(self.headers.get("Content-Length") or 0)
        payload = json.loads(self.rfile.read(length).decode() or "{}")
        query = str(payload.get("query") or "")
        cid = str(payload.get("conversation_id") or f"conv-{uuid.uuid4().hex[:12]}")
        if not query:
            self._send(400, {"error": "query is required"})
            return

        prompt = f"[eval_id:{cid}] {query}"
        argv = shlex.split(CMD)
        start = time.time()
        try:
            if MODE == "argv":
                proc = subprocess.run(argv + [prompt], text=True, capture_output=True, timeout=300)
            else:
                proc = subprocess.run(argv, input=prompt, text=True, capture_output=True, timeout=300)
        except Exception as exc:  # noqa: BLE001
            self._send(500, {"error": str(exc), "conversation_id": cid})
            return

        self._send(200, {
            "answer": proc.stdout.strip(),
            "stderr": proc.stderr[-2000:],
            "exit_code": proc.returncode,
            "conversation_id": cid,
            "blocked": False,
            "duration_ms": int((time.time() - start) * 1000),
        })


if __name__ == "__main__":
    print(f"cli-driver listening on http://localhost:{PORT} for: {CMD or '<unset>'}")
    ThreadingHTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
