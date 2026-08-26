## Context

`getSubjectName` maps `(topic, element, subjectNameStrategy, schema)` to a
registry subject name. `index.d.ts` declares all four params and three strategy
constants, but the Go `SubjectNameConfig` omits `schema`, and `getSubjectName`
implements only TopicNameStrategy — every other strategy silently falls through
to `{topic}-{element}`, so RecordNameStrategy returns a wrong (topic-based)
subject with no error. The record strategies need the schema's fully-qualified
record name, which for Avro is available from the parsed schema.

## Goals / Non-Goals

**Goals:**
- Implement TopicNameStrategy, RecordNameStrategy, TopicRecordNameStrategy.
- Derive the record full name from an Avro named schema.
- Fail loudly (error) when a record strategy can't produce a real name, instead
  of the current silent wrong result.

**Non-Goals:**
- JSON Schema / Protobuf record naming (no reliable record name without a type
  hint; out of scope for v1).
- Changing `index.d.ts` (schema + constants already declared).

## Decisions

- **Add `Schema string` (`js:"schema"`) to the Go `SubjectNameConfig`.** It is
  already a **mandatory** field in the `index.d.ts` type; the Go struct simply
  catches up so the always-supplied value crosses the bridge. The contract is
  unchanged — `schema` stays required for every call. TopicNameStrategy ignores
  it; the record strategies use it (and error if it is empty or not a parseable
  Avro named schema).

- **Record full name from Avro via the parsed schema.** Parse `schema` through
  the existing `sr.parsedAvro(...)` helper (so it benefits from the parsed-Avro
  cache) and type-assert the result to `avro.NamedSchema` (satisfied by Avro
  record/enum/fixed; it exposes `FullName()`). Use `FullName()`
  (namespace-qualified: `com.example.User`, or the bare name when there is no
  namespace — no leading dot). If parsing fails or the schema is not a named
  type, return an error. *Alternative:* also accept a JSON Schema `title`
  as the record name — rejected for v1: `getSubjectName` has no `schemaType`, so
  Avro-vs-JSON must be inferred, and JSON record naming is ambiguous; erroring is
  safer than guessing.

- **Strategy dispatch:**
  - empty → TopicNameStrategy (the community default).
  - `TopicNameStrategy` → `{topic}-{element}`.
  - `RecordNameStrategy` → `{fullname}`.
  - `TopicRecordNameStrategy` → `{topic}-{fullname}`.
  - anything else → error.
  Uses the existing `topicNameStrategy` / `recordNameStrategy` /
  `topicRecordNameStrategy` constants (the current code compares a hardcoded
  string literal — switch to the constants).

- **`GetSubjectName` gains error returns for the new failure modes.** It already
  returns `(string, error)`; the record strategies add "empty schema" and
  "non-Avro/unnamed schema" errors. The existing `topic`/`element` required-field
  checks stay unchanged (both are mandatory in the `index.d.ts` contract, so
  always supplied). Neither record strategy uses `element`. RecordNameStrategy
  additionally ignores `topic` (its subject is just the record full name), while
  TopicRecordNameStrategy *does* use `topic` (`{topic}-{fullname}`). Both still
  require `topic`/`element` to be present only to mirror the unchanged
  contract — `element` never feeds a record subject, and `topic` feeds only
  TopicRecordNameStrategy.

## Risks / Trade-offs

- **Behavior change for scripts already (mis)using record strategies.** Anyone
  who set RecordNameStrategy today silently got `{topic}-{element}`; now they get
  the real record name (or an error if no schema). This is the intended fix, but
  it changes output for those scripts. → Documented; it corrects a latent bug.
- **Avro-only record naming.** JSON/Protobuf users of record strategies get an
  error, not a subject. → Documented as a v1 limitation; TopicNameStrategy works
  for all.

## Open Questions

- None outstanding. (`element` stays required for all strategies to match the
  unchanged `index.d.ts` contract; record strategies ignore it. `schema` stays
  mandatory per the contract; record strategies error on an empty value.)
