/**
 * @module k6/x/kafka
 *
 * grafana/xk6-kafka is the official, Grafana-owned, pure-Go k6 extension for
 * load testing Apache Kafka. It enables k6 users to load test Apache Kafka:
 * producing and consuming messages, managing topics, authenticating, and
 * working with Schema Registry.
 *
 * It is a 100% pure-Go extension (`CGO_ENABLED=0`): no C toolchain and no
 * `confluentinc/librdkafka`, so it runs in lightweight containers, strict
 * CI/CD pipelines, and Grafana Cloud k6.
 *
 * This API aims to be a near-drop-in replacement for community
 * `mostafa/xk6-kafka` v1 scripts: same import and API shape, so common
 * producer, consumer, admin, auth, and Schema Registry scripts run with little
 * or no change. The goal is familiarity, not a behavior-identical guarantee:
 * some legacy tuning options have no equivalent and are accepted but
 * ignored, so behavior can differ in edge cases.
 *
 * @remarks
 * All grouped values (compression codecs, SASL mechanisms, schema types, and so
 * on) are exported as individual top-level constants, e.g.
 * `import { CODEC_SNAPPY, SCHEMA_TYPE_AVRO } from "k6/x/kafka"`. Each group also
 * has a matching type alias (such as {@link COMPRESSION_CODECS}) for typing
 * configuration fields.
 *
 * @see {@link https://github.com/grafana/xk6-kafka}
 */

/* -------------------------------------------------------------------------- *
 * Shared: timeouts, authentication, and TLS                                  *
 * -------------------------------------------------------------------------- */

/** One nanosecond. */
export const NANOSECOND: 1;
/** One microsecond (1,000 nanoseconds). */
export const MICROSECOND: 1000;
/** One millisecond (1,000,000 nanoseconds). */
export const MILLISECOND: 1000000;
/** One second (1,000,000,000 nanoseconds). */
export const SECOND: 1000000000;
/** One minute (60 seconds). */
export const MINUTE: 60000000000;
/** One hour (60 minutes). */
export const HOUR: 3600000000000;
/** Time units, expressed in nanoseconds, for use in timeouts. */
export type TIME =
  | typeof NANOSECOND
  | typeof MICROSECOND
  | typeof MILLISECOND
  | typeof SECOND
  | typeof MINUTE
  | typeof HOUR;

/** No SASL authentication. */
export const NONE: "none";
/** SASL/PLAIN: username and password sent as plaintext (use with TLS). */
export const SASL_PLAIN: "sasl_plain";
/** SASL/SCRAM using SHA-256. */
export const SASL_SCRAM_SHA256: "sasl_scram_sha256";
/** SASL/SCRAM using SHA-512. */
export const SASL_SCRAM_SHA512: "sasl_scram_sha512";
/**
 * SASL over a TLS connection. Uses the PLAIN mechanism with the configured
 * `username`/`password`, and requires TLS to be enabled.
 */
export const SASL_SSL: "sasl_ssl";
/**
 * SASL with AWS IAM credentials (AWS MSK).
 * @remarks Not yet implemented: selecting this mechanism currently errors. It is
 * deferred to a dedicated change (it needs an AWS credential provider).
 */
export const SASL_AWS_IAM: "sasl_aws_iam";
/**
 * SASL mechanisms for authenticating to Kafka.
 * @remarks
 * Covers no-auth, PLAIN, SCRAM (SHA-256/512), SSL, and AWS IAM. The Azure Entra
 * OAuth mechanism (`SASL_AZURE_ENTRA`) from later community versions, used for
 * Azure Event Hub, is not part of this v1-compatible surface yet.
 */
export type SASL_MECHANISMS =
  | typeof NONE
  | typeof SASL_PLAIN
  | typeof SASL_SCRAM_SHA256
  | typeof SASL_SCRAM_SHA512
  | typeof SASL_SSL
  | typeof SASL_AWS_IAM;

/** SASL configuration for authenticating to Kafka. Which fields you set depends on the chosen `algorithm`. */
export interface SASLConfig {
  /** Username for the SASL mechanism. */
  username?: string;
  /** Password for the SASL mechanism. */
  password?: string;
  /**
   * Which SASL mechanism to use.
   * @defaultValue {@link NONE}
   */
  algorithm?: SASL_MECHANISMS;
  /** AWS profile name, used only when `algorithm` is {@link SASL_AWS_IAM}. */
  awsProfile?: string;
}

/** TLS 1.0. */
export const TLS_1_0: "tlsv1.0";
/** TLS 1.1. */
export const TLS_1_1: "tlsv1.1";
/** TLS 1.2. */
export const TLS_1_2: "tlsv1.2";
/** TLS 1.3. */
export const TLS_1_3: "tlsv1.3";
/** TLS versions for creating a secure communication channel with Kafka. */
export type TLS_VERSIONS =
  | typeof TLS_1_0
  | typeof TLS_1_1
  | typeof TLS_1_2
  | typeof TLS_1_3;

/** TLS configuration for creating a secure communication channel with Kafka. Set only the fields you need. */
export interface TLSConfig {
  /** Enable TLS for the connection. */
  enableTls?: boolean;
  /** Skip server certificate and hostname verification (insecure; use only for testing). */
  insecureSkipTlsVerify?: boolean;
  /** Minimum acceptable TLS version. */
  minVersion?: TLS_VERSIONS;
  /** Client certificate in PEM format, for mutual TLS. */
  clientCertPem?: string;
  /** Client private key in PEM format, for mutual TLS. */
  clientKeyPem?: string;
  /** Server CA certificate in PEM format, used to verify the broker. */
  serverCaPem?: string;
}

/**
 * Configuration for loading a Java KeyStore (JKS) from a file.
 *
 * @remarks
 * Set `clientKeyAlias` to extract a client key + certificate chain (keystore),
 * and/or `serverCaAlias` to extract a server CA (truststore); either may be
 * omitted, so a keystore-only or truststore-only JKS works.
 */
export interface JKSConfig {
  /** Path to the JKS keystore file. */
  path: string;
  /** Password protecting the keystore. */
  password: string;
  /**
   * Alias of the client certificate within the keystore.
   * @remarks Accepted for compatibility but currently not used: the client
   * certificate chain is taken from the private-key entry (`clientKeyAlias`).
   */
  clientCertAlias?: string;
  /** Alias of the client private key (and its certificate chain). Omit for a truststore-only keystore. */
  clientKeyAlias?: string;
  /** Password protecting the client private key. */
  clientKeyPassword?: string;
  /** Alias of the server CA certificate. Omit for a keystore-only keystore. */
  serverCaAlias?: string;
}

/** Certificates and key extracted from a JKS keystore, in PEM format. */
export interface JKS {
  /** Client certificate chain in PEM format. */
  clientCertsPem: string[];
  /** Client private key in PEM format. */
  clientKeyPem: string;
  /** Server CA certificate in PEM format. */
  serverCaPem: string;
}

/**
 * Load a JKS keystore from a file. Returns PEM strings you can feed
 * into a {@link TLSConfig} for mutual TLS.
 * @param jksConfig - JKS configuration.
 * @returns JKS client and server certificates and private key.
 * @remarks Call this in the init context (it reads the keystore through k6's
 * file system). Only the JKS format is supported; PKCS#12 keystores are not.
 * @example
 * ```javascript
 * const jks = LoadJKS({
 *  path: "/path/to/keystore.jks",
 *  password: "password",
 *  clientCertAlias: "localhost",
 *  clientKeyAlias: "localhost",
 *  clientKeyPassword: "password",
 *  serverCaAlias: "ca",
 * });
 * ```
 */
export function LoadJKS(jksConfig: JKSConfig): JKS;

/* -------------------------------------------------------------------------- *
 * Messages (shared by producer and consumer)                                 *
 * -------------------------------------------------------------------------- */

/**
 * A Kafka message. The same shape is used both for messages you produce and
 * for messages you read back, so every field is optional.
 *
 * @remarks
 * When producing, you typically set only `key` and/or `value` (both accept a
 * plain string or a `Uint8Array`). When consuming, every field is populated by
 * Kafka — `key` and `value` come back as `Uint8Array`, `headers` as a plain
 * object, and `time` as an RFC3339 string.
 */
export interface Message {
  /** Topic the message belongs to. When producing, overrides the writer's default topic. */
  topic?: string;
  /** Partition the message belongs to. Set by Kafka on read. */
  partition?: number;
  /** Position of the message within its partition. Set by Kafka on read. */
  offset?: number;
  /** Highest offset available in the partition at read time (how far behind you are). Set by Kafka on read. */
  highWaterMark?: number;
  /**
   * Message key. When producing, pass a plain string (sent as UTF-8 bytes) or a
   * `Uint8Array` for raw bytes. The key decides the target partition for
   * hashing balancers. Returned as a `Uint8Array` when consuming.
   */
  key?: string | Uint8Array;
  /**
   * Message value (the payload). When producing, pass a plain string (sent as
   * UTF-8 bytes) or a `Uint8Array` for raw bytes. Returned as a `Uint8Array`
   * when consuming.
   */
  value?: string | Uint8Array;
  /**
   * Message headers as a plain object, e.g. `{ myKey: "myValue" }`. Returned as
   * a plain object when consuming.
   */
  headers?: Record<string, any>;
  /**
   * Message timestamp. When producing, pass a JavaScript `Date` (for example
   * `new Date()`); it is converted to a Kafka timestamp automatically. Returned
   * as an RFC3339 string when consuming.
   */
  time?: Date | string;
}

/* -------------------------------------------------------------------------- *
 * Producer (Writer)                                                          *
 * -------------------------------------------------------------------------- */

/** Gzip compression. */
export const CODEC_GZIP: "gzip";
/** Snappy compression. */
export const CODEC_SNAPPY: "snappy";
/** LZ4 compression. */
export const CODEC_LZ4: "lz4";
/** Zstandard (zstd) compression. */
export const CODEC_ZSTD: "zstd";
/** Compression codecs for compressing messages when producing to a topic or reading from it. */
export type COMPRESSION_CODECS =
  | typeof CODEC_GZIP
  | typeof CODEC_SNAPPY
  | typeof CODEC_LZ4
  | typeof CODEC_ZSTD;

/** Spread messages evenly across partitions in round-robin order. */
export const BALANCER_ROUND_ROBIN: "balancer_roundrobin";
/** Route each message to the partition with the fewest bytes written. */
export const BALANCER_LEAST_BYTES: "balancer_leastbytes";
/** Hash the message key to pick a partition (compatible with the Java client default). */
export const BALANCER_HASH: "balancer_hash";
/**
 * Pick a partition using a CRC32 hash of the key.
 * @remarks
 * Accepted for v1 compatibility but currently has no equivalent on the pure-Go
 * (franz-go) path, so it may not be honored yet. This may change as the
 * implementation matures.
 */
export const BALANCER_CRC32: "balancer_crc32";
/** Pick a partition using the murmur2 hash of the key (compatible with librdkafka/Java). */
export const BALANCER_MURMUR2: "balancer_murmur2";
/** Balancers for distributing messages to partitions. */
export type BALANCERS =
  | typeof BALANCER_ROUND_ROBIN
  | typeof BALANCER_LEAST_BYTES
  | typeof BALANCER_HASH
  | typeof BALANCER_CRC32
  | typeof BALANCER_MURMUR2;

/**
 * Custom balancer callback that returns the destination partition index for a
 * message key, given the number of partitions. Use it as the
 * {@link WriterConfig.balancer} when the built-in {@link BALANCERS} don't fit.
 * @remarks
 * A custom balancer may not be honored on the pure-Go (franz-go) path yet; this
 * may change as the implementation matures.
 */
export type BalancerFunction = (key: Uint8Array, partitionCount: number) => number;

/**
 * Writer configuration for producing messages to a topic.
 *
 * @remarks
 * Only `brokers` is required. `topic` is the default produce topic (optional if
 * each message sets its own `topic`). Every other field is optional and falls
 * back to a sensible default, so most scripts set just `brokers` and `topic`.
 */
export interface WriterConfig {
  /** Broker addresses to connect to, e.g. `["localhost:9092"]`. Required. */
  brokers: string[];
  /**
   * Default topic to produce to, used for any message that does not set its own
   * `topic`. Optional only if every message sets its own `topic`.
   */
  topic?: string;
  /**
   * Create the topic automatically if it does not exist yet.
   *
   * @remarks
   * The topic is created with the broker's default settings (often a single
   * partition) and you cannot choose the replication factor or other options.
   * For full control, create the topic with {@link Connection.createTopic} in
   * the test's `setup()` function instead.
   * @defaultValue `false`
   */
  autoCreateTopic?: boolean;
  /**
   * How messages are spread across partitions: either a built-in
   * {@link BALANCERS} value or your own {@link BalancerFunction}.
   * @defaultValue {@link BALANCER_LEAST_BYTES}
   * @remarks
   * On the pure-Go (franz-go) path, round-robin, hash, and murmur2 map directly;
   * least-bytes maps approximately. {@link BALANCER_CRC32} and a custom
   * {@link BalancerFunction} may not be honored yet. This may change as the
   * implementation matures.
   */
  balancer?: BALANCERS | BalancerFunction;
  /**
   * How many times to retry sending a message before giving up. Must be `>= 0`:
   * `0` disables retries; leave unset to use the client default.
   */
  maxAttempts?: number;
  /**
   * How many messages to group together before sending them as one batch.
   * Larger batches are more efficient but add a little latency.
   *
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which batches by size and time rather than message count.
   * This may change as the implementation matures.
   */
  batchSize?: number;
  /**
   * Maximum size of a batch in bytes. A batch is sent once it reaches this size.
   *
   * @remarks
   * This is the raw, uncompressed sum of all keys and values in the batch, not
   * the per-message size. A batch can be rejected for exceeding the broker's
   * limit (1 MB by default) even when each individual message is small — raise
   * this (e.g. to 2 MB) when producing many messages per call.
   */
  batchBytes?: number;
  /**
   * How long to wait for a batch to fill up before sending it anyway, in
   * nanoseconds (see {@link TIME}, e.g. `200 * MILLISECOND`).
   */
  batchTimeout?: number;
  /**
   * Originally the socket read timeout, in nanoseconds (see {@link TIME}).
   * @remarks Accepted for compatibility but ignored on the pure-Go (franz-go)
   * path, which manages socket read deadlines internally with no equivalent knob.
   */
  readTimeout?: number;
  /**
   * How many broker acknowledgements to wait for before treating a write as
   * done. This is the durability vs. speed trade-off.
   *
   * @remarks
   * - `-1`: wait for all in-sync replicas (safest, slowest).
   * - `0`: don't wait at all (fastest, messages may be lost).
   * - `1`: wait only for the partition leader (in between).
   *
   * Only `-1`, `0`, and `1` are valid; other values are rejected.
   * @defaultValue `-1`
   */
  requiredAcks?: number;
  /**
   * Maximum time to wait for a produce request to be acknowledged by the broker,
   * in nanoseconds (see {@link TIME}).
   * @remarks On the pure-Go (franz-go) path this is the produce request timeout;
   * it approximates the v1 socket write timeout.
   */
  writeTimeout?: number;
  /**
   * Compression to apply to produced messages. Also a way to fit more data
   * under the broker's message-size limit and avoid `MessageTooLargeError`.
   * @defaultValue no compression
   */
  compression?: COMPRESSION_CODECS;
  /** SASL authentication settings. Leave unset to connect without authentication. */
  sasl?: SASLConfig;
  /** TLS settings. Leave unset to connect without TLS. */
  tls?: TLSConfig;
  /**
   * Log low-level connection activity to the k6 output. Useful for debugging
   * connection problems; noisy in normal runs.
   * @defaultValue `false`
   * @remarks Accepted but not yet wired on the pure-Go path; currently has no effect.
   */
  connectLogger?: boolean;
}

/** Configuration for producing messages to a topic. */
export interface ProduceConfig {
  /** Messages to produce in this call. */
  messages: Message[];
}

/**
 * Writer writes messages to Kafka.
 * @example
 *
 * ```javascript
 * // In init context
 * const writer = new Writer({
 *   brokers: ["localhost:9092"],
 *   topic: "my-topic",
 *   autoCreateTopic: true,
 * });
 *
 * // In VU code (default function)
 * writer.produce({
 *   messages: [
 *     {
 *       key: "key",
 *       value: "value",
 *     }
 *   ]
 * });
 *
 * // In teardown function
 * writer.close();
 * ```
 */
export class Writer {
  /**
   * Create a new Writer.
   * @param writerConfig - Writer configuration.
   */
  constructor(writerConfig: WriterConfig);
  /**
   * Send one or more messages to Kafka. Call this from the VU context (the
   * default function, or `setup`/`teardown`) — not the init context.
   * @param produceConfig - The messages to send.
   * @example
   * ```javascript
   * writer.produce({
   *   messages: [
   *     { key: "user-1", value: "hello", headers: { source: "k6" } },
   *     { value: "no key needed" },
   *   ],
   * });
   * ```
   */
  produce(produceConfig: ProduceConfig): void;
  /**
   * Close the writer and free its connections. Call this in `teardown()`.
   */
  close(): void;
}

/* -------------------------------------------------------------------------- *
 * Consumer (Reader)                                                          *
 * -------------------------------------------------------------------------- */

/** Start consuming from the earliest available offset (default). */
export const START_OFFSETS_FIRST_OFFSET: "start_offsets_first_offset";
/** Start consuming from the latest available offset. */
export const START_OFFSETS_LAST_OFFSET: "start_offsets_last_offset";
/**
 * Backward-compatibility constant for the earliest available offset.
 * @see {@link START_OFFSETS_FIRST_OFFSET}
 */
export const FIRST_OFFSET: "start_offsets_first_offset";
/**
 * Backward-compatibility constant for the latest available offset.
 * @see {@link START_OFFSETS_LAST_OFFSET}
 */
export const LAST_OFFSET: "start_offsets_last_offset";
/** Where a consumer group starts reading when it has no committed offset yet. */
export type START_OFFSETS =
  | typeof START_OFFSETS_FIRST_OFFSET
  | typeof START_OFFSETS_LAST_OFFSET;

/** Return all records, including those from uncommitted transactions. */
export const ISOLATION_LEVEL_READ_UNCOMMITTED: "isolation_level_read_uncommitted";
/** Return only records from committed transactions. */
export const ISOLATION_LEVEL_READ_COMMITTED: "isolation_level_read_committed";
/** Isolation levels control the visibility of transactional records. */
export type ISOLATION_LEVEL =
  | typeof ISOLATION_LEVEL_READ_UNCOMMITTED
  | typeof ISOLATION_LEVEL_READ_COMMITTED;

/** Assign contiguous partition ranges per topic to group members. */
export const GROUP_BALANCER_RANGE: "group_balancer_range";
/** Assign partitions to group members in round-robin order. */
export const GROUP_BALANCER_ROUND_ROBIN: "group_balancer_round_robin";
/**
 * Prefer assigning partitions to members in the same rack to reduce cross-rack traffic.
 * @remarks
 * Accepted for v1 compatibility but has no equivalent on the pure-Go (franz-go)
 * path, so it is ignored; a group with no other balancer uses the `range`
 * default. This may change as the implementation matures.
 */
export const GROUP_BALANCER_RACK_AFFINITY: "group_balancer_rack_affinity";
/** Consumer group balancing strategies for consuming messages. */
export type GROUP_BALANCERS =
  | typeof GROUP_BALANCER_RANGE
  | typeof GROUP_BALANCER_ROUND_ROBIN
  | typeof GROUP_BALANCER_RACK_AFFINITY;

/**
 * Configuration for creating a {@link Reader} instance.
 *
 * @remarks
 * Required: `brokers`, plus a consumption target — either a `groupID` together
 * with `groupTopics` (or `topic`) for consumer-group consumption, or a `topic`
 * for direct single-partition consumption (`partition` defaults to `0`; use a
 * consumer group to read multiple partitions). `groupTopics` alone, without a
 * `groupID`, is not valid. The many tuning fields below are optional and default
 * to sensible values.
 */
export interface ReaderConfig {
  /** Broker addresses to connect to, e.g. `["localhost:9092"]`. Required. */
  brokers: string[];
  /**
   * Consumer group ID. When set, several readers sharing the same ID split the
   * topic's partitions between them and Kafka remembers their progress
   * (committed offsets). Leave empty to read on your own without a group.
   */
  groupID?: string;
  /** Topics the consumer group reads from. When using a group, set this or `topic`. */
  groupTopics?: string[];
  /**
   * Single topic to read from. Used without a consumer group, and also as the
   * group's topic when `groupID` is set but `groupTopics` is not.
   */
  topic?: string;
  /**
   * Specific partition to read from in direct mode; defaults to `0` when omitted.
   * @remarks Ignored when `groupID` is set, because the group assigns partitions for you.
   */
  partition?: number;
  /**
   * Size of the internal buffer that holds fetched messages before you consume them.
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which manages fetch buffering differently. This may change
   * as the implementation matures.
   */
  queueCapacity?: number;
  /**
   * Smallest amount of data, in bytes, the broker should return per fetch. The
   * broker waits until this much is available (or `maxWait` passes). Higher
   * values are more efficient but add latency.
   */
  minBytes?: number;
  /** Largest amount of data, in bytes, to return per fetch (e.g. `1048576` for 1 MB). */
  maxBytes?: number;
  /**
   * Timeout for a single read batch, in nanoseconds (see {@link TIME}).
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, where `maxWait` covers fetch waiting. This may change as
   * the implementation matures.
   */
  readBatchTimeout?: number;
  /**
   * Longest time to wait for new messages, as a duration string (e.g. `"200ms"`,
   * `"5s"`). It bounds both the broker fetch wait and how long a single
   * `consume` call blocks.
   *
   * @remarks
   * Defaults to about 5 seconds (the franz-go fetch-wait default). On
   * low-throughput topics, raise it so `consume` waits long enough for messages
   * instead of timing out.
   */
  maxWait?: string;
  /**
   * Interval at which the reader computes consumer lag, in nanoseconds (see {@link TIME}).
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which has no equivalent. This may change as the
   * implementation matures.
   */
  readLagInterval?: number;
  /**
   * Consumer group rebalancing strategies, in priority order.
   * @defaultValue {@link GROUP_BALANCER_RANGE}
   */
  groupBalancers?: GROUP_BALANCERS[];
  /** Interval between consumer group heartbeats, in nanoseconds (see {@link TIME}). */
  heartbeatInterval?: number;
  /** Interval at which offsets are committed, in nanoseconds (see {@link TIME}). */
  commitInterval?: number;
  /**
   * Interval at which the reader checks for partition changes, in nanoseconds (see {@link TIME}).
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which refreshes metadata automatically. This may change as
   * the implementation matures.
   */
  partitionWatchInterval?: number;
  /**
   * Watch for partition count changes and trigger a rebalance when they occur.
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which detects partition changes automatically. This may
   * change as the implementation matures.
   */
  watchPartitionChanges?: boolean;
  /**
   * Consumer group session timeout, in nanoseconds (see {@link TIME}).
   * @remarks Set this explicitly if group consumption ends abruptly under load.
   */
  sessionTimeout?: number;
  /** Consumer group rebalance timeout, in nanoseconds (see {@link TIME}). */
  rebalanceTimeout?: number;
  /**
   * Backoff before retrying a failed join-group request, in nanoseconds (see {@link TIME}).
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which uses a single client-wide retry backoff. This may
   * change as the implementation matures.
   */
  joinGroupBackoff?: number;
  /**
   * Retention time for committed offsets, in nanoseconds (see {@link TIME}).
   * @remarks
   * Accepted for v1 compatibility but currently ignored on the pure-Go
   * (franz-go) path, which has no equivalent. This may change as the
   * implementation matures.
   */
  retentionTime?: number;
  /**
   * Where to start reading when there is no position to resume from: the
   * earliest or the latest message. Applies to a consumer group with no
   * committed offset, and to a direct-partition reader that does not set an
   * explicit `offset`.
   * @defaultValue {@link START_OFFSETS_FIRST_OFFSET}
   */
  startOffset?: START_OFFSETS;
  /**
   * Minimum backoff between read retries, in nanoseconds (see {@link TIME}).
   * @remarks Accepted but ignored on the pure-Go (franz-go) path, which has no read-specific backoff knob.
   */
  readBackoffMin?: number;
  /**
   * Maximum backoff between read retries, in nanoseconds (see {@link TIME}).
   * @remarks Accepted but ignored on the pure-Go (franz-go) path, which has no read-specific backoff knob.
   */
  readBackoffMax?: number;
  /**
   * Enable the underlying client's connection logger.
   * @remarks Accepted but not yet wired on the pure-Go (franz-go) path; currently has no effect.
   */
  connectLogger?: boolean;
  /**
   * How many times to retry a read before returning an error. Must be `>= 0`:
   * `0` disables retries; leave unset to use the client default.
   */
  maxAttempts?: number;
  /**
   * Whether to include messages from transactions that are not yet committed.
   * @defaultValue {@link ISOLATION_LEVEL_READ_UNCOMMITTED}
   */
  isolationLevel?: ISOLATION_LEVEL;
  /**
   * Exact offset to start reading from when reading a single partition without
   * a group. Takes precedence over {@link startOffset} (the symbolic
   * earliest/latest setting) for a direct-partition reader.
   * @remarks Use `0` to start from the beginning, `-1` for the latest message, or any positive number for a specific offset. Values below `-1` are rejected.
   */
  offset?: number;
  /** SASL authentication settings. Leave unset to connect without authentication. */
  sasl?: SASLConfig;
  /** TLS settings. Leave unset to connect without TLS. */
  tls?: TLSConfig;
}

/** Configuration for the {@link Reader.consume} method. */
export interface ConsumeConfig {
  /**
   * Maximum number of messages to return from this call. `consume` returns once
   * it has this many messages; if the reader's `maxWait` passes first, see
   * `expectTimeout`.
   */
  limit: number;
  /**
   * If `true`, timestamps on returned messages keep nanosecond precision
   * instead of being rounded.
   * @defaultValue `false`
   */
  nanoPrecision?: boolean;
  /**
   * If `true`, return whatever messages were collected so far when `maxWait`
   * passes, instead of waiting for the full `limit`. If `false` (the default),
   * a `maxWait` timeout before `limit` messages arrive throws instead.
   * @defaultValue `false`
   */
  expectTimeout?: boolean;
}

/**
 * Reader reads messages from Kafka.
 * @example
 *
 * ```javascript
 * // In init context
 * const reader = new Reader({
 *   brokers: ["localhost:9092"],
 *   topic: "my-topic",
 * });
 *
 * // In VU code (default function)
 * const messages = reader.consume({ limit: 10 });
 *
 * // In teardown function
 * reader.close();
 * ```
 */
export class Reader {
  /**
   * Create a new Reader.
   * @param readerConfig - Reader configuration.
   */
  constructor(readerConfig: ReaderConfig);
  /**
   * Read up to `limit` messages from Kafka. Call this from the VU context (the
   * default function, or `setup`/`teardown`) — not the init context. By default
   * it throws if `maxWait` passes before `limit` messages arrive; set
   * `expectTimeout` to return the partial (or empty) batch instead.
   * @param consumeConfig - How many messages to read and how to wait.
   * @returns The messages read.
   * @example
   * ```javascript
   * const messages = reader.consume({ limit: 10 });
   * console.log(`got ${messages.length} messages`);
   * ```
   */
  consume(consumeConfig: ConsumeConfig): Message[];
  /**
   * Close the reader and free its connections. Call this in `teardown()`.
   */
  close(): void;
}

/* -------------------------------------------------------------------------- *
 * Admin (topic management)                                                   *
 * -------------------------------------------------------------------------- */

/** Configuration for creating a {@link Connection} instance for working with topics. */
export interface ConnectionConfig {
  /** Broker address to connect to, e.g. `localhost:9092`. Required. */
  address: string;
  /** SASL authentication settings. Leave unset to connect without authentication. */
  sasl?: SASLConfig;
  /** TLS settings. Leave unset to connect without TLS. */
  tls?: TLSConfig;
}

/** Replica assignment among Kafka brokers for a topic's partitions. */
export interface ReplicaAssignment {
  /** Partition the assignment applies to. */
  partition: number;
  /** Broker IDs hosting replicas for this partition. */
  replicas: number[];
}

/** A single topic-level configuration entry to set on a topic. */
export interface ConfigEntry {
  /** Configuration key, e.g. `retention.ms`. */
  configName: string;
  /** Configuration value. */
  configValue: string;
}

/** Topic configuration for creating a new topic. */
export interface TopicConfig {
  /** Name of the topic to create. Required. */
  topic: string;
  /**
   * How many partitions the topic has. More partitions allow more parallel
   * consumers. Ignored when `replicaAssignments` is set — then the number of
   * assignment entries determines the partition count.
   * @defaultValue `1`
   */
  numPartitions?: number;
  /**
   * How many brokers keep a copy of each partition. Use `1` for a single-broker
   * dev cluster. Ignored when `replicaAssignments` is set.
   * @defaultValue `1`
   */
  replicationFactor?: number;
  /**
   * Place specific partitions on specific brokers yourself. When set, the
   * assignment list fully determines the topic's layout — the number of entries
   * is the partition count and each entry's `replicas` is that partition's
   * placement — so both `numPartitions` and `replicationFactor` are ignored.
   * The entries must describe a contiguous layout: one per partition with
   * `partition` IDs covering exactly `0` to `N-1` (unique, no gaps).
   */
  replicaAssignments?: ReplicaAssignment[];
  /** Extra topic settings, e.g. retention. See {@link ConfigEntry}. */
  configEntries?: ConfigEntry[];
}

/**
 * Connection connects to Kafka for working with topics.
 * @example
 *
 * ```javascript
 * // In init context
 * const connection = new Connection({
 *   address: "localhost:9092",
 * });
 *
 * // In VU code (default function)
 * const topics = connection.listTopics();
 *
 * // In teardown function
 * connection.close();
 * ```
 */
export class Connection {
  /**
   * Create a new Connection.
   * @param connectionConfig - Connection configuration.
   */
  constructor(connectionConfig: ConnectionConfig);
  /**
   * Create a new topic. Call this from the VU context (the default function, or
   * `setup`/`teardown`) — not the init context, and not after `close`. Throws if
   * `topic` is empty or a `replicaAssignments` partition is negative or
   * duplicated (checked before contacting the broker), and if the broker rejects
   * the request (for example, the topic already exists).
   * @param topicConfig - Name, partition count, replication, and any topic settings.
   * @remarks
   * Create topics in the test's `setup()` function so the topic exists before
   * any VU starts producing or consuming. Creating topics from VU code can
   * cause race conditions.
   */
  createTopic(topicConfig: TopicConfig): void;
  /**
   * Delete a topic. Call this from the VU context (the default function, or
   * `setup`/`teardown`) — not the init context, and not after `close`. Throws if
   * `topic` is empty or the broker rejects the request. Kafka removes the topic
   * asynchronously, so it may stay visible in {@link Connection.listTopics} for a
   * short while after this returns.
   * @param topic - Name of the topic to delete.
   */
  deleteTopic(topic: string): void;
  /**
   * List the names of the cluster's topics. Internal topics (such as
   * `__consumer_offsets`) are excluded. Call this from the VU context (the
   * default function, or `setup`/`teardown`) — not the init context, and not
   * after `close`.
   * @returns Topic names.
   */
  listTopics(): string[];
  /**
   * Close the connection and free its resources.
   */
  close(): void;
}

/* -------------------------------------------------------------------------- *
 * Schema Registry and serialization                                          *
 * -------------------------------------------------------------------------- */

/** Treat the data as a UTF-8 string. */
export const SCHEMA_TYPE_STRING: "STRING";
/** Treat the data as a raw byte array. */
export const SCHEMA_TYPE_BYTES: "BYTES";
/** Apache Avro schema. */
export const SCHEMA_TYPE_AVRO: "AVRO";
/** JSON Schema. */
export const SCHEMA_TYPE_JSON: "JSON";
/**
 * Protocol Buffers schema.
 * @remarks
 * Not supported in v1: Protobuf serdes arrived in the community v2 surface,
 * which is out of scope for this v1-compatible extension. The constant is
 * exported for source compatibility, but `serialize`/`deserialize` throw for
 * `PROTOBUF`. Use {@link SCHEMA_TYPE_AVRO} or {@link SCHEMA_TYPE_JSON}.
 */
export const SCHEMA_TYPE_PROTOBUF: "PROTOBUF";
/** Schema types used in identifying schema and data type in serdes. */
export type SCHEMA_TYPES =
  | typeof SCHEMA_TYPE_STRING
  | typeof SCHEMA_TYPE_BYTES
  | typeof SCHEMA_TYPE_AVRO
  | typeof SCHEMA_TYPE_JSON
  | typeof SCHEMA_TYPE_PROTOBUF;

/** The message key. */
export const KEY: "key";
/** The message value. */
export const VALUE: "value";
/** Element types for publishing schemas to Schema Registry. */
export type ELEMENT_TYPES = typeof KEY | typeof VALUE;

/** Derive the subject from the topic name: `<topic>-key` or `<topic>-value`. */
export const TOPIC_NAME_STRATEGY: "TopicNameStrategy";
/** Derive the subject from the record (schema) name. */
export const RECORD_NAME_STRATEGY: "RecordNameStrategy";
/** Derive the subject from both the topic and the record name. */
export const TOPIC_RECORD_NAME_STRATEGY: "TopicRecordNameStrategy";
/** Subject name strategy for storing a schema in Schema Registry. */
export type SUBJECT_NAME_STRATEGY =
  | typeof TOPIC_NAME_STRATEGY
  | typeof RECORD_NAME_STRATEGY
  | typeof TOPIC_RECORD_NAME_STRATEGY;

/** Basic authentication for connecting to Schema Registry. */
export interface BasicAuth {
  /** Username for Schema Registry basic auth. */
  username: string;
  /** Password for Schema Registry basic auth. */
  password: string;
}

/** Schema Registry connection settings, used to store and look up schemas. */
export interface SchemaRegistryConfig {
  /** Schema Registry URL, e.g. `http://localhost:8081`. Required. */
  url: string;
  /** Keep a local copy of fetched schemas so repeated lookups skip the network. */
  enableCaching?: boolean;
  /** Username and password, if the registry needs basic auth. Leave unset otherwise. */
  basicAuth?: BasicAuth;
  /** TLS settings for connecting to the registry. Leave unset for plain HTTP. */
  tls?: TLSConfig;
}

/**
 * A schema reference: the `import` statement for Protobuf and the `$ref` field
 * for JSON Schema.
 */
export interface Reference {
  /** Reference name, e.g. the imported file or type name. */
  name: string;
  /** Subject under which the referenced schema is registered. */
  subject: string;
  /** Version of the referenced schema. */
  version: number;
}

/**
 * A schema used by the {@link SchemaRegistry} client to encode and decode
 * messages. You get one back from `getSchema` / `createSchema` and pass it to
 * `serialize` / `deserialize`.
 */
export interface Schema {
  /**
   * The registry subject this schema is stored under, e.g. `my-topic-value`.
   * Required when loading from or registering to Schema Registry; omit for
   * inline (no-registry) serdes that pass the schema text directly.
   */
  subject?: string;
  /** The schema definition itself, as a string (Avro/JSON/Protobuf text). Required when registering or for inline serdes. */
  schema?: string;
  /** Which schema format this is. Required when registering. */
  schemaType?: SCHEMA_TYPES;
  /**
   * Accepted but ignored in v1. Caching is controlled at the client level via
   * `SchemaRegistryConfig.enableCaching`, not per schema.
   */
  enableCaching?: boolean;
  /** Numeric ID assigned by Schema Registry. Set for you when the schema is registered or loaded. */
  id?: number;
  /** Version number of the schema within its subject. Set for you on load; pass to request a specific version. */
  version?: number;
  /** Other schemas this one depends on. */
  references?: Reference[];
}

/** Configuration for computing a Schema Registry subject name. */
export interface SubjectNameConfig {
  /** The schema definition as a string. */
  schema: string;
  /** Topic the schema is associated with. */
  topic: string;
  /** Whether the schema applies to the message key or value. */
  element: ELEMENT_TYPES;
  /** Strategy used to derive the subject name. */
  subjectNameStrategy: SUBJECT_NAME_STRATEGY;
}

/** Bundles data with the schema and type used to encode or decode it. */
export interface Container {
  /** The value to encode, or (for `deserialize`) the bytes to decode. */
  data: any;
  /** Which schema format to use. */
  schemaType: SCHEMA_TYPES;
  /**
   * Schema describing the data. Omit for the simple `STRING` and `BYTES`
   * types, which need no registered schema.
   */
  schema?: Schema;
}

/**
 * SchemaRegistry is a client for Schema Registry and handles serdes.
 * @example
 *
 * ```javascript
 * // In init context
 * const writer = new Writer({
 *   brokers: ["localhost:9092"],
 *   topic: "my-topic",
 *   autoCreateTopic: true,
 * });
 *
 * const schemaRegistry = new SchemaRegistry({
 *   url: "http://localhost:8081",
 * });
 *
 * const keySchema = schemaRegistry.createSchema({
 *   subject: "my-topic-key",
 *   schema: "...",
 *   schemaType: SCHEMA_TYPE_AVRO,
 * });
 *
 * const valueSchema = schemaRegistry.createSchema({
 *   subject: "my-topic-value",
 *   schema: "...",
 *   schemaType: SCHEMA_TYPE_AVRO,
 * });
 *
 * // In VU code (default function)
 * writer.produce({
 *   messages: [
 *     {
 *       key: schemaRegistry.serialize({
 *         data: "key",
 *         schema: keySchema,
 *         schemaType: SCHEMA_TYPE_AVRO
 *       }),
 *       value: schemaRegistry.serialize({
 *         data: "value",
 *         schema: valueSchema,
 *         schemaType: SCHEMA_TYPE_AVRO
 *       }),
 *     }
 *   ]
 * });
 * ```
 */
export class SchemaRegistry {
  /**
   * Create a new SchemaRegistry client.
   * @param schemaRegistryConfig - Schema Registry configuration. Omit it entirely
   * to use standalone serdes (string, bytes, or inline Avro/JSON schemas) without
   * talking to a Schema Registry server.
   */
  constructor(schemaRegistryConfig?: SchemaRegistryConfig);
  /**
   * Load an existing schema from Schema Registry.
   * @param schema - Requires `subject`; optionally `version` to pick a specific version (otherwise the latest).
   * @returns The full schema, including its registry `id`.
   */
  getSchema(schema: Schema): Schema;
  /**
   * Register a new schema (or a new version of one) in Schema Registry and
   * return it with its assigned `id`. If the schema already exists, the
   * existing one is returned.
   * @param schema - Requires `subject`, `schema`, and `schemaType`.
   * @returns The registered schema, including its registry `id`.
   */
  createSchema(schema: Schema): Schema;
  /**
   * Work out the registry subject name for a topic and key/value, using the
   * chosen naming strategy. Handy when you let the registry auto-register schemas.
   * @param subjectNameConfig - Topic, key-or-value, strategy, and schema.
   * @returns The subject name, e.g. `my-topic-value`.
   */
  getSubjectName(subjectNameConfig: SubjectNameConfig): string;
  /**
   * Turn data into the byte array that goes into a message `key` or `value`.
   * Use this before producing.
   * @param container - The data plus the schema and schema type to encode it with.
   * @returns The encoded bytes.
   * @remarks
   * For Avro unions, you can pass the value directly (e.g. `field: "x"` or
   * `field: null`) or wrapped (e.g. `field: { string: "x" }`). Declare `null`
   * first in a union (`["null", "string"]`) per Avro convention.
   * @example
   * ```javascript
   * const value = schemaRegistry.serialize({
   *   data: { firstName: "John", lastName: "Doe" },
   *   schema: valueSchema,
   *   schemaType: SCHEMA_TYPE_AVRO,
   * });
   * ```
   */
  serialize(container: Container): Uint8Array;
  /**
   * Turn the bytes from a consumed message `key` or `value` back into data.
   * Use this after consuming.
   * @param container - The bytes (in `data`) plus the schema and schema type to decode them with.
   * @returns The decoded value: a string, a byte array, or a JSON object.
   * @example
   * ```javascript
   * const user = schemaRegistry.deserialize({
   *   data: messages[0].value,
   *   schema: valueSchema,
   *   schemaType: SCHEMA_TYPE_AVRO,
   * });
   * ```
   */
  deserialize(container: Container): any;
}
