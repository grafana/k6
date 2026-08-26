## Why

`getSubjectName` advertises three subject-naming strategies via exported
constants (`TOPIC_NAME_STRATEGY`, `RECORD_NAME_STRATEGY`,
`TOPIC_RECORD_NAME_STRATEGY`) and `index.d.ts` requires a `schema` on
`SubjectNameConfig`, but the implementation only handles TopicNameStrategy and
silently collapses the record strategies to `{topic}-{element}`. A script asking
for RecordNameStrategy gets a wrong subject name with no error — a compatibility
footgun. This change implements the record strategies.

## What Changes

- `getSubjectName` implements all three strategies:
  - **TopicNameStrategy** (and the default when unset): `{topic}-{element}`
    (`{topic}-key` / `{topic}-value`).
  - **RecordNameStrategy**: the schema's fully-qualified record name.
  - **TopicRecordNameStrategy**: `{topic}-{record-fullname}`.
- The record strategies derive the name from the `schema` string (already
  declared on `SubjectNameConfig` in `index.d.ts`). A new `Schema` field is added
  to the Go `SubjectNameConfig` so the value crosses the JS bridge.
- Record-name derivation supports **Avro** named schemas (record/enum/fixed via
  the parsed schema's full name); a schema that is not a parseable Avro named
  type yields a clear error. JSON Schema / Protobuf record naming is **not
  supported in v1** (documented) — record strategies error rather than guess.
- An unrecognized (non-empty) `subjectNameStrategy` yields an error instead of a
  silent fallback; an empty strategy defaults to TopicNameStrategy.
- No `index.d.ts` change: `SubjectNameConfig.schema` and the strategy constants
  are already declared.

## Capabilities

### New Capabilities
<!-- None: this is behavior of the existing schema-registry capability. -->

### Modified Capabilities
- `schema-registry`: the "Subject naming strategy" requirement now defines
  RecordNameStrategy and TopicRecordNameStrategy (Avro), how the `schema`
  parameter is used, and the error behavior for unsupported schemas/strategies.

## Impact

- **Code**: `pkg/kafka/schema_registry.go` — add `Schema` to `SubjectNameConfig`;
  `getSubjectName` implements the record strategies via a record-full-name
  helper (parsing the schema through the existing `parsedAvro` cache).
- **Contract**: none (`index.d.ts` already declares `schema` + the constants).
- **Docs**: README/compatibility note that record strategies are Avro-only in v1.
- **Tests**: each strategy; Avro full-name (namespaced and bare, plus a
  non-record named schema like enum/fixed); empty schema; non-Avro/unnamed
  schema; unknown strategy; TopicName ignoring schema.
