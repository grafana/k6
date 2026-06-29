// sigil.js — a Sigil source for k6/experimental/ageval.
//
// Fetches an agent's recorded run (its generations) from a Sigil (AI
// observability) instance and maps it to the canonical ageval recorded-run shape
//   { input, output, toolCalls: [{ name, input, output }], model, usage, durationMs, tags }
// so you can evaluate REAL production/agent runs with `new AgentTestCase(...)` +
// `judge()` from k6/experimental/ageval. Sigil is just the trajectory source;
// all assertion/scoring is done by the native module.
//
// Two read modes:
//   - cloud: Grafana app-plugin proxy — SIGIL_BASE + ABT_SIGIL_TOKEN
//       SIGIL_BASE=https://<your-stack>/api/plugins/grafana-sigil-app/resources/query
//   - local: Sigil HTTP API (/api/v1, tenant header) — SIGIL_URL + SIGIL_TENANT
//
// It also provides askAgent()/convId() for "drive a local agent, then read its
// run back" flows (see local/ examples).
import http from "k6/http";
import exec from "k6/execution";
import encoding from "k6/encoding";
import { sleep } from "k6";

export const AGENT_URL = __ENV.AGENT_URL || "http://localhost:8000";
export const SIGIL_BASE = __ENV.SIGIL_BASE || ""; // cloud app-plugin proxy base (no default)
export const SIGIL_URL = __ENV.SIGIL_URL || "http://localhost:8090"; // local Sigil
export const SIGIL_TENANT = __ENV.SIGIL_TENANT || "fake";

// --- driving a (local) agent ----------------------------------------------

export function convId(tag) {
  return `${tag}-${__VU}-${__ITER}-${Date.now()}`;
}

export function askAgent(query, conversationId, path) {
  return http.post(
    `${AGENT_URL}${path || "/api/agent"}`,
    JSON.stringify({ query, conversation_id: conversationId }),
    { headers: { "Content-Type": "application/json" }, tags: { name: "agent_query" }, timeout: "180s" },
  );
}

// --- reading generations from Sigil ----------------------------------------

function cloudHeaders() {
  const h = { Accept: "application/json" };
  if (__ENV.SIGIL_AUTHORIZATION) h["Authorization"] = __ENV.SIGIL_AUTHORIZATION;
  else if (__ENV.ABT_SIGIL_TOKEN) h["Authorization"] = `Bearer ${__ENV.ABT_SIGIL_TOKEN}`;
  if (__ENV.SIGIL_TENANT) h["X-Scope-OrgID"] = __ENV.SIGIL_TENANT;
  return h;
}

function requireSigilBase() {
  if (!SIGIL_BASE) {
    exec.test.abort(
      "SIGIL_BASE is not set. Point it at your Sigil app-plugin proxy base URL, e.g. " +
        "https://<your-stack>/api/plugins/grafana-sigil-app/resources/query, and set ABT_SIGIL_TOKEN. See README.",
    );
  }
}

function normGen(g) {
  const x = Object.assign({}, g || {});
  if (!x.id && x.generation_id) x.id = x.generation_id;
  return x;
}

// cloud: fetch a full conversation object from the app-plugin proxy.
export function getConversation(conversationId) {
  requireSigilBase();
  const res = http.get(`${SIGIL_BASE}/conversations/${encodeURIComponent(conversationId)}`, {
    headers: cloudHeaders(),
    tags: { name: "sigil_get_conversation" },
  });
  if (res.status !== 200) return null;
  try { return JSON.parse(res.body); } catch (_e) { return null; }
}

export function listConversations(limit) {
  requireSigilBase();
  const q = limit ? `?limit=${limit}` : "";
  const res = http.get(`${SIGIL_BASE}/conversations${q}`, {
    headers: cloudHeaders(),
    tags: { name: "sigil_list_conversations" },
  });
  if (res.status !== 200) return [];
  try { return JSON.parse(res.body).items || []; } catch (_e) { return []; }
}

// cloud: poll for a conversation's generations (Sigil ingestion is async).
export function cloudGenerations(conversationId, attempts) {
  const tries = attempts || 1;
  for (let i = 0; i < tries; i++) {
    const c = getConversation(conversationId);
    if (c && (c.generations || []).length > 0) return (c.generations || []).map(normGen);
    if (i < tries - 1) sleep(0.5);
  }
  return [];
}

// local: single read of /api/v1 generations.
function localGenerationsOnce(conversationId) {
  const res = http.get(`${SIGIL_URL}/api/v1/conversations/${conversationId}`, {
    headers: { "X-Scope-OrgID": SIGIL_TENANT },
  });
  if (res.status !== 200) return [];
  try { return (JSON.parse(res.body).generations || []).map(normGen); } catch (_e) { return []; }
}

export function fetchGenerations(conversationId, attempts) {
  const tries = attempts || 20;
  for (let i = 0; i < tries; i++) {
    const g = localGenerationsOnce(conversationId);
    if (g.length > 0) return g;
    sleep(0.5);
  }
  return [];
}

export function peekGenerations(conversationId) {
  return localGenerationsOnce(conversationId);
}

function localConversations() {
  const res = http.get(`${SIGIL_URL}/api/v1/conversations`, { headers: { "X-Scope-OrgID": SIGIL_TENANT } });
  if (res.status !== 200) return [];
  try {
    return (JSON.parse(res.body).items || []).slice()
      .sort((a, b) => String(b.last_generation_at || "").localeCompare(String(a.last_generation_at || "")));
  } catch (_e) { return []; }
}

// local: correlate a run to its generations by a unique marker in the prompt
// (for agents — e.g. chat UIs — that cannot accept a conversation_id).
export function findGenerationsByMarker(marker) {
  for (let attempt = 0; attempt < 40; attempt++) {
    for (const c of localConversations().slice(0, 10)) {
      const cid = c.id || c.conversation_id;
      if (!cid) continue;
      const g = localGenerationsOnce(cid);
      if (g.some((x) => generationText(x).includes(marker))) return g;
    }
    sleep(0.5);
  }
  return [];
}

// --- mapping Sigil generations -> ageval recorded-run ----------------------

// Sigil stores tool-call args base64-encoded in `input_json`.
export function decodeToolInput(toolCall) {
  if (toolCall.input && typeof toolCall.input === "object") return toolCall.input;
  if (typeof toolCall.input_json === "string" && toolCall.input_json.length) {
    try { return JSON.parse(encoding.b64decode(toolCall.input_json, "std", "s")); } catch (_e) { return {}; }
  }
  return {};
}

function ms(v) { const p = Date.parse(v || ""); return Number.isNaN(p) ? 0 : p; }
function startMs(g) { return ms(g.started_at || g.created_at); }
function endMs(g) { return ms(g.completed_at || g.created_at || g.started_at); }

function textOfMessage(msg) {
  const out = [];
  for (const p of msg.parts || []) if (typeof p.text === "string" && p.text) out.push(p.text);
  return out.join("\n");
}

// All readable text in a generation (input + output, incl. tool_result content).
export function generationText(g) {
  const out = [];
  for (const section of [g.input || [], g.output || []]) {
    for (const m of section) {
      for (const p of m.parts || []) {
        if (typeof p.text === "string") out.push(p.text);
        if (p.tool_result && typeof p.tool_result.content === "string") out.push(p.tool_result.content);
      }
    }
  }
  return out.join("\n");
}

// Map tool_use_id -> tool result content, so each tool call can carry its output.
function toolResultsById(gens) {
  const map = {};
  for (const g of gens) {
    for (const section of [g.input || [], g.output || []]) {
      for (const m of section) {
        for (const p of m.parts || []) {
          const tr = p.tool_result;
          if (tr && typeof tr.content === "string") {
            const id = tr.tool_use_id || tr.id || tr.tool_call_id;
            if (id) map[id] = tr.content;
          }
        }
      }
    }
  }
  return map;
}

// recordedRun maps a set of Sigil generations to the ageval recorded-run shape.
export function recordedRun(generations, opts) {
  const o = opts || {};
  const gens = (generations || []).slice().sort((a, b) => startMs(a) - startMs(b));
  const results = toolResultsById(gens);

  const toolCalls = [];
  for (const g of gens) {
    for (const m of g.output || []) {
      for (const p of m.parts || []) {
        if (p.tool_call && p.tool_call.name) {
          toolCalls.push({
            name: p.tool_call.name,
            input: decodeToolInput(p.tool_call),
            output: results[p.tool_call.id || ""] || "",
          });
        }
      }
    }
  }

  let output = "";
  for (let i = gens.length - 1; i >= 0; i--) {
    const t = (gens[i].output || []).map(textOfMessage).filter(Boolean).join("\n");
    if (t) { output = t; break; }
  }

  const input = gens.length ? (gens[0].input || []).map(textOfMessage).filter(Boolean).join("\n") : "";

  let inTok = 0, outTok = 0;
  for (const g of gens) {
    const u = g.usage || {};
    inTok += parseInt(u.input_tokens || "0", 10);
    outTok += parseInt(u.output_tokens || "0", 10);
  }

  const model = gens.length ? (gens[0].model && gens[0].model.name) || gens[0].response_model || "" : "";
  const durationMs = gens.length ? Math.max(0, endMs(gens[gens.length - 1]) - startMs(gens[0])) : 0;

  return {
    input,
    output,
    toolCalls,
    model,
    usage: { inputTokens: inTok, outputTokens: outTok },
    durationMs,
    tags: o.tags || {},
  };
}

// sigilRun: cloud convenience — fetch a conversation by id and map it to a
// recorded run ready for `new AgentTestCase(sigilRun(id))`.
export function sigilRun(conversationId, opts) {
  const o = opts || {};
  return recordedRun(cloudGenerations(conversationId, o.attempts || 10), { tags: o.tags });
}

// --- host specs (for model-aware dashboards) -------------------------------
// k6 can't introspect the host's hardware, so specs are supplied via env vars
// (pass them with -e, e.g. `-e HOST_CPU="$(sysctl -n machdep.cpu.brand_string)"`).
// Call once (e.g. from setup()): logs a single `host_specs {json}` line that the
// LLM testing plugin parses and shows in the run's Environment panel. No-op when
// no HOST_*/MODEL_HOST vars are set.
const HOST_SPEC_ENV = {
  cpu: "HOST_CPU",
  mem_gb: "HOST_MEM_GB",
  gpu: "HOST_GPU",
  os: "HOST_OS",
  model_host: "MODEL_HOST",
};

export function reportHostSpecs() {
  const specs = {};
  for (const key in HOST_SPEC_ENV) {
    const value = __ENV[HOST_SPEC_ENV[key]];
    if (value) {
      specs[key] = value;
    }
  }
  if (Object.keys(specs).length > 0) {
    console.log("host_specs " + JSON.stringify(specs));
  }
}
