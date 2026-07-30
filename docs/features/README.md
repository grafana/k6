# Feature briefs

A feature brief is a short Markdown document that describes a user problem and explains why it matters. You submit it as a pull request and the team reviews it before any implementation work begins. Keeping briefs in the repo means the whole proposal history lives alongside the code.

## When to write one

Write a brief for new features or significant changes to existing behaviour. You do **not** need one for bug fixes, small improvements, or refactors.

## How to submit

1. Copy [`000-TEMPLATE.md`](000-TEMPLATE.md) to a new file named `NNN-short-title.md`, where `NNN` is the next available number (e.g. `007-websocket-multiplexing.md`).
2. Fill in every section, deleting the HTML comment guidance as you go.
3. Open a PR that adds only that file under `docs/features/`.

## Review process

Briefs are reviewed as pull requests, with discussion happening in the PR comments. Once a brief is merged we treat it as final, so any follow-up changes go into a new brief.

## Constraints

| Constraint | Limit |
|---|---|
| Total prose | 1,200 words (soft target: 800) |
| Problem statement | ≤ 3 sentences |
| "Cost of inaction" section | Required |

The word count is enforced in CI. Template headings and HTML comments do not count towards it.

---

For general contribution guidelines see [`CONTRIBUTING.md`](../../CONTRIBUTING.md).
