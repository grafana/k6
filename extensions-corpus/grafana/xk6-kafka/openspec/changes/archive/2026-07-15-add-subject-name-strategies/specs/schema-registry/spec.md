## MODIFIED Requirements

### Requirement: Subject naming strategy

`schemaRegistry.getSubjectName(subjectNameConfig)` SHALL return the subject name for a topic
and key-or-value designation. Per the `index.d.ts` contract the parameter MUST include `topic`
(topic name), `element` (KEY or VALUE constant), `subjectNameStrategy` (naming strategy
constant), and `schema` (schema definition string) — all four are always supplied. `element` is
used only by TopicNameStrategy; `schema` is used only by the record strategies (the others
ignore it).

The supported strategies are:

- **TopicNameStrategy** (also the default when `subjectNameStrategy` is empty): returns
  `{topic}-key` or `{topic}-value` per `element`; ignores `schema`.
- **RecordNameStrategy**: returns the schema's fully-qualified record name (e.g.
  `com.example.User`).
- **TopicRecordNameStrategy**: returns `{topic}-{record-fullname}`.

The record name is derived from an **Avro** named schema — record, enum, or fixed — via its
fully-qualified name. If `schema` is empty, or is not a parseable Avro named schema, the record
strategies SHALL return an error rather than guess. JSON Schema and Protobuf record naming are
not supported in v1. An unrecognized (non-empty) `subjectNameStrategy` SHALL return an error.

#### Scenario: TopicNameStrategy for value

- **WHEN** getSubjectName is called with topic="my-topic", element=VALUE, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-value" is returned

#### Scenario: TopicNameStrategy for key

- **WHEN** getSubjectName is called with topic="my-topic", element=KEY, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-key" is returned

#### Scenario: Empty strategy defaults to TopicNameStrategy

- **WHEN** getSubjectName is called with topic="my-topic", element=VALUE, and an empty subjectNameStrategy
- **THEN** "my-topic-value" is returned

#### Scenario: TopicNameStrategy ignores the schema

- **WHEN** getSubjectName is called with TopicNameStrategy, topic="my-topic", element=VALUE, and
  an empty (or arbitrary) `schema`
- **THEN** "my-topic-value" is returned regardless of the `schema` value

#### Scenario: RecordNameStrategy returns the record full name

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  record schema named `User` in namespace `com.example`
- **THEN** "com.example.User" is returned (`topic` and `element` are still supplied per the
  contract, but do not affect a RecordNameStrategy subject)

#### Scenario: RecordNameStrategy with a record that has no namespace

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  record named `User` with no namespace
- **THEN** "User" is returned (the bare name, with no leading dot)

#### Scenario: TopicRecordNameStrategy combines topic and record full name

- **WHEN** getSubjectName is called with topic="my-topic",
  subjectNameStrategy=TOPIC_RECORD_NAME_STRATEGY, and an Avro record schema `com.example.User`
- **THEN** "my-topic-com.example.User" is returned

#### Scenario: RecordNameStrategy works for other named Avro schemas

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  enum (or fixed) named `com.example.Color`
- **THEN** "com.example.Color" is returned (any named Avro schema, not only records)

#### Scenario: Record strategy with an empty schema errors

- **WHEN** getSubjectName is called with a record strategy and an empty `schema`
- **THEN** an error is returned

#### Scenario: Record strategy with a non-Avro or unnamed schema errors

- **WHEN** getSubjectName is called with a record strategy and a `schema` that is not a
  parseable Avro named schema (e.g. a JSON Schema, or an Avro primitive)
- **THEN** an error is returned

#### Scenario: Unknown strategy errors

- **WHEN** getSubjectName is called with a non-empty `subjectNameStrategy` that is not one of
  the three supported constants
- **THEN** an error is returned
