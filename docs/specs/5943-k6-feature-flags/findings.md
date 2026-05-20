# Findings — SPEC.md vs SPEC.caveman.md

This document collects independent reviews from multiple AI agents comparing the two specification variants for the **k6 Feature Flags** epic:

- `SPEC.md` — full natural-language specification (737 lines).
- `SPEC.caveman.md` — caveman-compressed variant of the same spec (736 lines), in which narrative prose drops articles and filler words while normative tables, code blocks, the Mermaid diagram, and most `MUST`/`MUST NOT` clauses are preserved verbatim.

Each agent records a section below explaining which document it prefers and why. The table is a quick overview; the per-agent sections give reasoning.

## Quick overview

| AI agent              | Preferred document
|-----------------------|--------------------
| Claude                | `SPEC.md`
| Codex                 | `SPEC.md`
| Copilot (with Gemini) | `SPEC.md`

---

## Claude

**Preferred:** `SPEC.md`.

### Why

1. **Normative documents trade tokens for unambiguity, and a spec is a normative document.** The whole point of `SPEC.md` is to be the contract that downstream PLAN/CODE/REVIEW artifacts can cite line-by-line. Dropping articles and conjunctions in narrative passages shaves a small percentage of tokens but introduces ambiguities that a future reader (human or model) has to re-parse — e.g. *"Engine emit `ERROR` ([FR-UNKNOWN-LOG]); tombstoned env var contribute nothing"* reads as a list of facts rather than a single causal sentence. In a requirements document that cost is real.

2. **Compression coverage is uneven, which is worse than no compression.** Looking at the two files side by side: the Mermaid diagram, every table cell, all `logrus` field schemas, the structured-field cross-reference table ([SPEC.caveman.md:344-355](SPEC.caveman.md#L344-L355)), the `FR-METRICS-METADATA` *Contract* block ([SPEC.caveman.md:418-427](SPEC.caveman.md#L418-L427)), the `FR-LEGACY-PHASES` table and most of its supporting prose ([SPEC.caveman.md:454-477](SPEC.caveman.md#L454-L477)), and the entire Appendix A are **byte-identical** to `SPEC.md`. Caveman style only really fires on the User Scenarios and overview prose. The result is a hybrid register — half terse fragments, half full English — which is more jarring to read than either pure form.

3. **Token savings are negligible.** Line counts differ by 1 (737 vs 736). The actual byte delta is small because the bulk of this spec is tables, code fences, and `MUST`-clauses that the caveman skill correctly refuses to touch. The remaining savings — perhaps 5–10% on the touched sections — do not justify the readability cost for a document that lives in `docs/specs/` and will be re-read by humans during planning, review, and onboarding.

4. **Cross-document linking is fragile under compression.** The spec already does a lot of internal-anchor work — `[FR-CONFIG-SURFACES](#fr-config-surfaces--…)` etc. Caveman-style abbreviation is most tempting around connective phrases ("see also", "as defined in"), and those are exactly the phrases that load-bear when readers chase references. Keeping them in full English preserves the navigation experience.

5. **Audience mismatch for compression.** The compressed form earns its keep on ephemeral, model-only artifacts that get re-read by an LLM many times. A canonical spec checked into the repo is the opposite use case — written once, read by many readers (some of them human reviewers, some external contributors), and rarely re-loaded into a model's context all in one shot. The compression target is mismatched for a shipped repo document.

### Caveats

- The caveman variant is **not wrong**. It preserves all normative substance; an engineer or LLM implementing the spec from it would arrive at the same design. The objection is to register and audience fit, not to correctness.
- If the project wanted to minimize repo size or LLM input cost during planning, the more impactful lever would be splitting the spec into a normative core (FR/NFR sections + Glossary) and a non-normative narrative (Scenarios + Appendix A), not compressing both in the same file.
- Carrying two near-identical files in the repo invites drift. The same forcing-function logic the spec itself prescribes for flag rot in [NFR-MAINTAINABILITY](SPEC.md#nfr-maintainability--maintainability) does not exist for docs — if both are kept, a process needs to guarantee they stay in sync.

### Recommendation

Keep `SPEC.md` as the canonical document. If a caveman variant is useful for downstream agent workflows, prefer generating it on demand from the canonical source rather than committing two parallel files that can silently diverge.

---

## Codex

**Preferred:** `SPEC.md`.

### Why

1. **`SPEC.md` is easier to trust as a canonical spec.** The caveman version keeps the same structure, FR labels, tables, and most normative clauses, but it rewrites the narrative and acceptance prose into telegraphic English. That makes many statements slightly harder to parse on first read, especially in scenarios where sequence and causality matter.

2. **The savings are real but small relative to the readability hit.** Measured locally, `SPEC.md` is 10,392 words / 82,711 bytes, while `SPEC.caveman.md` is 9,785 words / 80,131 bytes. That is only about 5.8% fewer words and 3.1% fewer bytes, which is not much for a document meant to be read and reviewed by humans.

3. **Most of the spec's load-bearing content is already unchanged.** The Mermaid diagram, requirement tables, field schemas, and appendices are largely identical between the two files. The 274-line diff is mostly concentrated in overview/scenario prose, so the compressed version removes fluency from the parts humans rely on for orientation while leaving most of the token-heavy normative material intact.

4. **Acceptance criteria read better in full English.** In `SPEC.md`, acceptance bullets read like clear testable statements. In `SPEC.caveman.md`, lines such as "engine emit", "run continue", and "feature activate" are still understandable, but they feel more like notes than a polished contract. For implementation and review work, that weaker register is a step backward.

5. **The caveman version is better as a derived artifact than a peer source.** If the team wants a low-cost LLM input, generating a compressed copy from `SPEC.md` on demand makes sense. Keeping both checked in as near-peers creates maintenance overhead without enough upside.

### Recommendation

Keep `SPEC.md` as the canonical document. Use `SPEC.caveman.md` only as an optional generated aid for agent workflows, not as the preferred spec for humans reviewing or implementing the feature.

---

## Copilot (with Gemini)

**Preferred:** `SPEC.md`.

### Why

1. **Increased Cognitive Load:** The "caveman" style forces the reader to reconstruct the grammar and mentally fill in the missing articles and connectors. This overhead disrupts the flow, making it significantly more difficult for a human reader to quickly parse complex logical conditions.
2. **Inconsistent Register:** Because the tables, Mermaid diagrams, and normative keywords remain fully intact, `SPEC.caveman.md` frequently transitions between telegraphic phrases and formal specification sentences. This inconsistency is jarring and undermines the professional tone typically required for core architectural docs.
3. **Loss of Nuance and Precision:** In a software specification, exact wording is crucial to avoid ambiguity. The removal of filler words occasionally blurs the relationship between subjects and actions, potentially leading to misinterpretation during implementation.
4. **Negligible Token and Size Reduction:** The file size and token savings are exceptionally minor. Depriving the document of basic grammar does not provide enough optimization value to outweigh the damage done to the document's legibility.
5. **Human Readability First:** The primary consumers of source-controlled specification documents are human developers, maintainers, and reviewers. Readability and clarity should take priority over marginal storage or prompt-processing efficiency.

### Recommendation

Adopt `SPEC.md` as the single source of truth for this epic. `SPEC.caveman.md` adds unnecessary maintenance friction, introduces readability barriers, and yields almost no tangible benefit, so it should not be checked into the repository alongside the canonical file.
