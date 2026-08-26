## Why

`Connection` currently only connects and closes (#10). Tests need to create
their topics before producing/consuming (and clean them up after) instead of
relying on broker auto-creation, which is often disabled and never lets a test
pick partition count or replication. Topic admin also unlocks the deferred
produce-to-missing-topic integration test.

## What Changes

- `connection.createTopic(TopicConfig)` creates a topic from `topic`,
  `numPartitions`, `replicationFactor`, `replicaAssignments`, and `configEntries`.
  Partition count and replication factor default to `1` when unset; when
  `replicaAssignments` is given, the assignment list determines the topic's
  layout (entry count = partitions, each entry's `replicas` = placement), so
  `numPartitions` and `replicationFactor` are ignored.
- `connection.deleteTopic(topic)` deletes a topic by name.
- `connection.listTopics()` returns the names of all (non-internal) topics on the
  cluster.
- Input is validated locally before any broker round-trip (matching v1):
  `createTopic`/`deleteTopic` reject an empty `topic`, and `createTopic` rejects a
  `replicaAssignments` entry with a negative or duplicate `partition`.
- All three run in the VU context (the `Connection` is built and pinged in init)
  using the ambient k6 context with no extra client-side timeout, are rejected
  from the init context, and are rejected after `close`.

## Capabilities

### New Capabilities

- `admin`: `Connection` topic administration — `createTopic`, `deleteTopic`,
  and `listTopics`.

### Modified Capabilities

<!-- None. The `kafka-module` capability (Connection exists, connects, closes)
still holds; this adds topic-admin behavior as a new capability. -->

## Impact

- New admin code in `pkg/kafka` (topic create/delete/list) plus unit tests.
  Implemented with `twmb/franz-go`'s `kmsg` requests (`CreateTopicsRequest`,
  `DeleteTopicsRequest`, `MetadataRequest`) issued through the existing client —
  `kmsg` is already an indirect dependency via `kgo`, so **no new module** is
  added. `kadm`'s `CreateTopics` cannot express `replicaAssignments`, so the
  kmsg path is used uniformly.
- `Connection` gains the VU handle (like `Writer`/`Reader`) so admin calls can use
  `vu.Context()` and the init/closed guards; construction-time `Ping` is unchanged.
- An integration test exercising create → list → delete against a real broker,
  run by the integration CI job.
- `index.d.ts`: the admin surface (`createTopic`/`deleteTopic`/`listTopics`,
  `TopicConfig`, `ReplicaAssignment`, `ConfigEntry`) already exists; docs are
  refined only to state the partition/replication defaults, that
  `replicaAssignments` overrides `replicationFactor`, and that `listTopics`
  excludes internal topics. No type-shape changes.
