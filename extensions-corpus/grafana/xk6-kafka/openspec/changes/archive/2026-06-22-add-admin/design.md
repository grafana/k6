## Context

`Connection` is connect + close (#10). This change adds topic administration on
the same client/auth foundation: build the request, issue it through the existing
`kgo.Client`, and surface per-topic errors. Must conform to `index.d.ts` and stay
pure Go.

## Goals / Non-Goals

**Goals:**

- `createTopic`, `deleteTopic`, `listTopics` on `Connection`, run in the VU
  context with the init/closed guards.
- Honor `numPartitions`, `replicationFactor`, `replicaAssignments`, and
  `configEntries` per the `index.d.ts` contract.

**Non-Goals:**

- Altering topics, partitions reassignment, or describing/altering configs.
- Schema Registry and metrics — separate changes.

## Decisions

- **kmsg, not kadm.** Topic ops are issued as raw `kmsg` requests
  (`CreateTopicsRequest`, `DeleteTopicsRequest`, `MetadataRequest`) through the
  existing `client.Request(ctx, req)`. `kmsg` is already an indirect dependency
  via `kgo`, so this adds no module. `kadm.CreateTopics` only takes
  partitions/replication/configs and cannot express `replicaAssignments`; using
  `kmsg` uniformly keeps one code path and full contract coverage.
- **Create defaults & replica assignments.** When `replicaAssignments` is empty:
  `NumPartitions` defaults to `1` when unset (`<= 0`), `ReplicationFactor` to `1`.
  When `replicaAssignments` is set, the Kafka protocol requires both
  `NumPartitions` and `ReplicationFactor` to be `-1`; the assignment list alone
  determines the layout (entry count = partition count, each entry's `replicas` =
  that partition's placement). So the `numPartitions` and `replicationFactor`
  inputs are ignored in this mode — not validated or derived, simply unused —
  rather than reshaped from the assignments as community v1 did. Each assignment
  maps `partition` + `replicas` to a `CreateTopicsRequestTopicReplicaAssignment`.
  `configEntries` map to topic `Configs` (`configName`/`configValue`). The
  assignment `partition` IDs must form a contiguous layout `0..N-1` (validated
  locally: non-negative, unique, none `>= N`) so the entry count is the true
  partition count; sparse or out-of-range IDs are rejected rather than forwarded.
- **Delete.** A `DeleteTopicsRequest` for the single named topic. Both the
  legacy `TopicNames` (v0–v5) and the `Topics` (v6+) fields are set so franz-go's
  negotiated version carries the name on old and new brokers alike. It returns
  once the broker accepts the request; Kafka deletes asynchronously, so the
  method does not poll for the topic to vanish from metadata (matching v1, and
  avoiding a flaky wait loop).
- **List.** A `MetadataRequest` with no topics returns cluster metadata for all
  topics; internal topics (e.g. `__consumer_offsets`) are filtered out and the
  remaining names returned (nil topic names skipped). A top-level or per-topic
  error code is converted with `kerr.ErrorForCode` and thrown — the list is never
  silently truncated to hide a metadata failure (auth, rebootstrap on Kafka
  4.0+, etc.).
- **Per-topic errors.** Each response carries a per-topic `ErrorCode`; it is
  converted with `kerr.ErrorForCode` and a non-nil error is thrown (with the
  topic name and any broker message). A transport error from `Request` is thrown
  directly. `createTopic`/`deleteTopic` return nothing; success is the absence of
  a thrown error.
- **Local validation.** Before issuing any request, the input is validated and a
  bad value throws immediately (no broker round-trip), matching the community v1
  behavior so the API is a reliable contract rather than deferring to
  broker-specific failures: `createTopic`/`deleteTopic` reject an empty `topic`;
  `createTopic` rejects a `replicaAssignments` entry with a negative `partition`
  or a duplicate `partition`. Broker-side errors (already-exists, delete
  disabled, …) are still surfaced as thrown errors on top of this.
- **VU context & lifecycle.** `Connection` holds the VU (like `Writer`/`Reader`).
  Each admin call uses `vu.Context()` directly — the ambient k6 context, which
  cancels when the VU stops or the test ends — and adds no separate client-side
  deadline, so there is no hidden, non-configurable timeout (franz-go applies its
  own request retry/timeout internally). Admin calls throw from the init context
  (`vu.State() == nil`) and after `close` (`client == nil`). Construction-time
  `Ping` keeps using a background context with `pingTimeout`, since the VU context
  may not yet exist in init.

## Risks / Trade-offs

- **Delete-topic support** may be disabled on a cluster (`delete.topic.enable=
  false`); the broker then returns a per-topic error, which surfaces as a thrown
  error rather than a silent success. Acceptable — it reflects the cluster state.
- **List on large clusters** fetches full metadata. Acceptable for a load-test
  helper; topic-name listing is not a hot path.

## Open Questions

- None outstanding.
