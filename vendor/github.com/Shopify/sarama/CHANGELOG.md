# Changelog

#### Version 1.19.0 (2018-09-27)

New Features:
 - Implement a higher-level consumer group
   ([#1099](https://github.com/Shopify/sarama/pull/1099)).

Improvements:
 - Add support for Go 1.11
   ([#1176](https://github.com/Shopify/sarama/pull/1176)).

Bug Fixes:
 - Fix encoding of `MetadataResponse` with version 2 and higher
   ([#1174](https://github.com/Shopify/sarama/pull/1174)).
 - Fix race condition in mock async producer
   ([#1174](https://github.com/Shopify/sarama/pull/1174)).

#### Version 1.18.0 (2018-09-07)

New Features:
 - Make `Partitioner.RequiresConsistency` vary per-message
   ([#1112](https://github.com/Shopify/sarama/pull/1112)).
 - Add customizable partitioner
   ([#1118](https://github.com/Shopify/sarama/pull/1118)).
 - Add `ClusterAdmin` support for `CreateTopic`, `DeleteTopic`, `CreatePartitions`,
   `DeleteRecords`, `DescribeConfig`, `AlterConfig`, `CreateACL`, `ListAcls`, `DeleteACL`
   ([#1055](https://github.com/Shopify/sarama/pull/1055)).

Improvements:
 - Add support for Kafka 2.0.0
   ([#1149](https://github.com/Shopify/sarama/pull/1149)).
 - Allow setting `LocalAddr` when dialing an address to support multi-homed hosts
   ([#1123](https://github.com/Shopify/sarama/pull/1123)).
 - Simpler offset management
   ([#1127](https://github.com/Shopify/sarama/pull/1127)).

Bug Fixes:
 - Fix mutation of `ProducerMessage.MetaData` when producing to Kafka
   ([#1110](https://github.com/Shopify/sarama/pull/1110)).
 - Fix consumer block when response did not contain all the
   expected topic/partition blocks
   ([#1086](https://github.com/Shopify/sarama/pull/1086)).
 - Fix consumer block when response contains only constrol messages
   ([#1115](https://github.com/Shopify/sarama/pull/1115)).
 - Add timeout config for ClusterAdmin requests
   ([#1142](https://github.com/Shopify/sarama/pull/1142)).
 - Add version check when producing message with headers
   ([#1117](https://github.com/Shopify/sarama/pull/1117)).
 - Fix `MetadataRequest` for empty list of topics
   ([#1132](https://github.com/Shopify/sarama/pull/1132)).
 - Fix producer topic metadata on-demand fetch when topic error happens in metadata response
   ([#1125](https://github.com/Shopify/sarama/pull/1125)).

#### Version 1.17.0 (2018-05-30)

New Features:
 - Add support for gzip compression levels
   ([#1044](https://github.com/Shopify/sarama/pull/1044)).
 - Add support for Metadata request/response pairs versions v1 to v5
   ([#1047](https://github.com/Shopify/sarama/pull/1047),
    [#1069](https://github.com/Shopify/sarama/pull/1069)).
 - Add versioning to JoinGroup request/response pairs
   ([#1098](https://github.com/Shopify/sarama/pull/1098))
 - Add support for CreatePartitions, DeleteGroups, DeleteRecords request/response pairs
   ([#1065](https://github.com/Shopify/sarama/pull/1065),
    [#1096](https://github.com/Shopify/sarama/pull/1096),
    [#1027](https://github.com/Shopify/sarama/pull/1027)).
 - Add `Controller()` method to Client interface
   ([#1063](https://github.com/Shopify/sarama/pull/1063)).

Improvements:
 - ConsumerMetadataReq/Resp has been migrated to FindCoordinatorReq/Resp
   ([#1010](https://github.com/Shopify/sarama/pull/1010)).
 - Expose missing protocol parts: `msgSet` and `recordBatch`
   ([#1049](https://github.com/Shopify/sarama/pull/1049)).
 - Add support for v1 DeleteTopics Request
   ([#1052](https://github.com/Shopify/sarama/pull/1052)).
 - Add support for Go 1.10
   ([#1064](https://github.com/Shopify/sarama/pull/1064)).
 - Claim support for Kafka 1.1.0
   ([#1073](https://github.com/Shopify/sarama/pull/1073)).

Bug Fixes:
 - Fix FindCoordinatorResponse.encode to allow nil Coordinator
   ([#1050](https://github.com/Shopify/sarama/pull/1050),
    [#1051](https://github.com/Shopify/sarama/pull/1051)).
 - Clear all metadata when we have the latest topic info
   ([#1033](https://github.com/Shopify/sarama/pull/1033)).
 - Make `PartitionConsumer.Close` idempotent
   ([#1092](https://github.com/Shopify/sarama/pull/1092)).

#### Version 1.16.0 (2018-02-12)

New Features:
 - Add support for the Create/Delete Topics request/response pairs
   ([#1007](https://github.com/Shopify/sarama/pull/1007),
    [#1008](https://github.com/Shopify/sarama/pull/1008)).
 - Add support for the Describe/Create/Delete ACL request/response pairs
   ([#1009](https://github.com/Shopify/sarama/pull/1009)).
 - Add support for the five transaction-related request/response pairs
   ([#1016](https://github.com/Shopify/sarama/pull/1016)).

Improvements:
 - Permit setting version on mock producer responses
   ([#999](https://github.com/Shopify/sarama/pull/999)).
 - Add `NewMockBrokerListener` helper for testing TLS connections
   ([#1019](https://github.com/Shopify/sarama/pull/1019)).
 - Changed the default value for `Consumer.Fetch.Default` from 32KiB to 1MiB
   which results in much higher throughput in most cases
   ([#1024](https://github.com/Shopify/sarama/pull/1024)).
 - Reuse the `time.Ticker` across fetch requests in the PartitionConsumer to
   reduce CPU and memory usage when processing many partitions
   ([#1028](https://github.com/Shopify/sarama/pull/1028)).
 - Assign relative offsets to messages in the producer to save the brokers a
   recompression pass
   ([#1002](https://github.com/Shopify/sarama/pull/1002),
    [#1015](https://github.com/Shopify/sarama/pull/1015)).

Bug Fixes:
 - Fix producing uncompressed batches with the new protocol format
   ([#1032](https://github.com/Shopify/sarama/issues/1032)).
 - Fix consuming compacted topics with the new protocol format
   ([#1005](https://github.com/Shopify/sarama/issues/1005)).
 - Fix consuming topics with a mix of protocol formats
   ([#1021](https://github.com/Shopify/sarama/issues/1021)).
 - Fix consuming when the broker includes multiple batches in a single response
   ([#1022](https://github.com/Shopify/sarama/issues/1022)).
 - Fix detection of `PartialTrailingMessage` when the partial message was
   truncated before the magic value indicating its version
   ([#1030](https://github.com/Shopify/sarama/pull/1030)).
 - Fix expectation-checking in the mock of `SyncProducer.SendMessages`
   ([#1035](https://github.com/Shopify/sarama/pull/1035)).

#### Version 1.15.0 (2017-12-08)

New Features:
 - Claim official support for Kafka 1.0, though it did already work
   ([#984](https://github.com/Shopify/sarama/pull/984)).
 - Helper methods for Kafka version numbers to/from strings
   ([#989](https://github.com/Shopify/sarama/pull/989)).
 - Implement CreatePartitions request/response
   ([#985](https://github.com/Shopify/sarama/pull/985)).

Improvements:
 - Add error codes 45-60
   ([#986](https://github.com/Shopify/sarama/issues/986)).

Bug Fixes:
 - Fix slow consuming for certain Kafka 0.11/1.0 configurations
   ([#982](https://github.com/Shopify/sarama/pull/982)).
 - Correctly determine when a FetchResponse contains the new message format
   ([#990](https://github.com/Shopify/sarama/pull/990)).
 - Fix producing with multiple headers
   ([#996](https://github.com/Shopify/sarama/pull/996)).
 - Fix handling of truncated record batches
   ([#998](https://github.com/Shopify/sarama/pull/998)).
 - Fix leaking metrics when closing brokers
   ([#991](https://github.com/Shopify/sarama/pull/991)).

#### Version 1.14.0 (2017-11-13)

New Features:
 - Add support for the new Kafka 0.11 record-batch format, including the wire
   protocol and the necessary behavioural changes in the producer and consumer.
   Transactions and idempotency are not yet supported, but producing and
   consuming should work with all the existing bells and whistles (batching,
   compression, etc) as well as the new custom headers. Thanks to Vlad Hanciuta
   of Arista Networks for this work. Part of
   ([#901](https://github.com/Shopify/sarama/issues/901)).

Bug Fixes:
 - Fix encoding of ProduceResponse versions in test
   ([#970](https://github.com/Shopify/sarama/pull/970)).
 - Return partial replicas list when we have it
   ([#975](https://github.com/Shopify/sarama/pull/975)).

#### Version 1.13.0 (2017-10-04)

New Features:
 - Support for FetchRequest version 3
   ([#905](https://github.com/Shopify/sarama/pull/905)).
 - Permit setting version on mock FetchResponses
   ([#939](https://github.com/Shopify/sarama/pull/939)).
 - Add a configuration option to support storing only minimal metadata for
   extremely large clusters
   ([#937](https://github.com/Shopify/sarama/pull/937)).
 - Add `PartitionOffsetManager.ResetOffset` for backtracking tracked offsets
   ([#932](https://github.com/Shopify/sarama/pull/932)).

Improvements:
 - Provide the block-level timestamp when consuming compressed messages
   ([#885](https://github.com/Shopify/sarama/issues/885)).
 - `Client.Replicas` and `Client.InSyncReplicas` now respect the order returned
   by the broker, which can be meaningful
   ([#930](https://github.com/Shopify/sarama/pull/930)).
 - Use a `Ticker` to reduce consumer timer overhead at the cost of higher
   variance in the actual timeout
   ([#933](https://github.com/Shopify/sarama/pull/933)).

Bug Fixes:
 - Gracefully handle messages with negative timestamps
   ([#907](https://github.com/Shopify/sarama/pull/907)).
 - Raise a proper error when encountering an unknown message version
   ([#940](https://github.com/Shopify/sarama/pull/940)).

#### Version 1.12.0 (2017-05-08)

New Features:
 - Added support for the `ApiVersions` request and response pair, and Kafka
   version 0.10.2 ([#867](https://github.com/Shopify/sarama/pull/867)). Note
   that you still need to specify the Kafka version in the Sarama configuration
   for the time being.
 - Added a `Brokers` method to the Client which returns the complete set of
   active brokers ([#813](https://github.com/Shopify/sarama/pull/813)).
 - Added an `InSyncReplicas` method to the Client which returns the set of all
   in-sync broker IDs for the given partition, now that the Kafka versions for
   which this was misleading are no longer in our supported set
   ([#872](https://github.com/Shopify/sarama/pull/872)).
 - Added a `NewCustomHashPartitioner` method which allows constructing a hash
   partitioner with a custom hash method in case the default (FNV-1a) is not
   suitable
   ([#837](https://github.com/Shopify/sarama/pull/837),
    [#841](https://github.com/Shopify/sarama/pull/841)).

Improvements:
 - Recognize more Kafka error codes
   ([#859](https://github.com/Shopify/sarama/pull/859)).

Bug Fixes:
 - Fix an issue where decoding a malformed FetchRequest would not return the
   correct error ([#818](https://github.com/Shopify/sarama/pull/818)).
 - Respect ordering of group protocols in JoinGroupRequests. This fix is
   transparent if you're using the `AddGroupProtocol` or
   `AddGroupProtocolMetadata` helpers; otherwise you will need to switch from
   the `GroupProtocols` field (now deprecated) to use `OrderedGroupProtocols`
   ([#812](https://github.com/Shopify/sarama/issues/812)).
 - Fix an alignment-related issue with atomics on 32-bit architectures
   ([#859](https://github.com/Shopify/sarama/pull/859)).

#### Version 1.11.0 (2016-12-20)

_Important:_ As of Sarama 1.11 it is necessary to set the config value of
`Producer.Return.Successes` to true in order to use the SyncProducer. Previous
versions would silently override this value when instantiating a SyncProducer
which led to unexpected values and data races.

New Features:
 - Metrics! Thanks to Sébastien Launay for all his work on this feature
   ([#701](https://github.com/Shopify/sarama/pull/701),
    [#746](https://github.com/Shopify/sarama/pull/746),
    [#766](https://github.com/Shopify/sarama/pull/766)).
 - Add support for LZ4 compression
   ([#786](https://github.com/Shopify/sarama/pull/786)).
 - Add support for ListOffsetRequest v1 and Kafka 0.10.1
   ([#775](https://github.com/Shopify/sarama/pull/775)).
 - Added a `HighWaterMarks` method to the Consumer which aggregates the
   `HighWaterMarkOffset` values of its child topic/partitions
   ([#769](https://github.com/Shopify/sarama/pull/769)).

Bug Fixes:
 - Fixed producing when using timestamps, compression and Kafka 0.10
   ([#759](https://github.com/Shopify/sarama/pull/759)).
 - Added missing decoder methods to DescribeGroups response
   ([#756](https://github.com/Shopify/sarama/pull/756)).
 - Fix producer shutdown when `Return.Errors` is disabled
   ([#787](https://github.com/Shopify/sarama/pull/787)).
 - Don't mutate configuration in SyncProducer
   ([#790](https://github.com/Shopify/sarama/pull/790)).
 - Fix crash on SASL initialization failure
   ([#795](https://github.com/Shopify/sarama/pull/795)).

#### Version 1.10.1 (2016-08-30)

Bug Fixes:
 - Fix the documentation for `HashPartitioner` which was incorrect
   ([#717](https://github.com/Shopify/sarama/pull/717)).
 - Permit client creation even when it is limited by ACLs
   ([#722](https://github.com/Shopify/sarama/pull/722)).
 - Several fixes to the consumer timer optimization code, regressions introduced
   in v1.10.0. Go's timers are finicky
   ([#730](https://github.com/Shopify/sarama/pull/730),
    [#733](https://github.com/Shopify/sarama/pull/733),
    [#734](https://github.com/Shopify/sarama/pull/734)).
 - Handle consuming compressed relative offsets with Kafka 0.10
   ([#735](https://github.com/Shopify/sarama/pull/735)).

#### Version 1.10.0 (2016-08-02)

_Important:_ As of Sarama 1.10 it is necessary to tell Sarama the version of
Kafka you are running against (via the `config.Version` value) in order to use
features that may not be compatible with old Kafka versions. If you don't
specify this value it will default to 0.8.2 (the minimum supported), and trying
to use more recent features (like the offset manager) will fail with an error.

_Also:_ The offset-manager's behaviour has been changed to match the upstream
java consumer (see [#705](https://github.com/Shopify/sarama/pull/705) and
[#713](https://github.com/Shopify/sarama/pull/713)). If you use the
offset-manager, please ensure that you are committing one *greater* than the
last consumed message offset or else you may end up consuming duplicate
messages.

New Features:
 - Support for Kafka 0.10
   ([#672](https://github.com/Shopify/sarama/pull/672),
    [#678](https://github.com/Shopify/sarama/pull/678),
    [#681](https://github.com/Shopify/sarama/pull/681), and others).
 - Support for configuring the target Kafka version
   ([#676](https://github.com/Shopify/sarama/pull/676)).
 - Batch producing support in the SyncProducer
   ([#677](https://github.com/Shopify/sarama/pull/677)).
 - Extend producer mock to allow setting expectations on message contents
   ([#667](https://github.com/Shopify/sarama/pull/667)).

Improvements:
 - Support `nil` compressed messages for deleting in compacted topics
   ([#634](https://github.com/Shopify/sarama/pull/634)).
 - Pre-allocate decoding errors, greatly reducing heap usage and GC time against
   misbehaving brokers ([#690](https://github.com/Shopify/sarama/pull/690)).
 - Re-use consumer expiry timers, removing one allocation per consumed message
   ([#707](https://github.com/Shopify/sarama/pull/707)).

Bug Fixes:
 - Actually default the client ID to "sarama" like we say we do
   ([#664](https://github.com/Shopify/sarama/pull/664)).
 - Fix a rare issue where `Client.Leader` could return the wrong error
   ([#685](https://github.com/Shopify/sarama/pull/685)).
 - Fix a possible tight loop in the consumer
   ([#693](https://github.com/Shopify/sarama/pull/693)).
 - Match upstream's offset-tracking behaviour
   ([#705](https://github.com/Shopify/sarama/pull/705)).
 - Report UnknownTopicOrPartition errors from the offset manager
   ([#706](https://github.com/Shopify/sarama/pull/706)).
 - Fix possible negative partition value from the HashPartitioner
   ([#709](https://github.com/Shopify/sarama/pull/709)).

#### Version 1.9.0 (2016-05-16)

New Features:
 - Add support for custom offset manager retention durations
   ([#602](https://github.com/Shopify/sarama/pull/602)).
 - Publish low-level mocks to enable testing of third-party producer/consumer
   implementations ([#570](https://github.com/Shopify/sarama/pull/570)).
 - Declare support for Golang 1.6
   ([#611](https://github.com/Shopify/sarama/pull/611)).
 - Support for SASL plain-text auth
   ([#648](https://github.com/Shopify/sarama/pull/648)).

Improvements:
 - Simplified broker locking scheme slightly
   ([#604](https://github.com/Shopify/sarama/pull/604)).
 - Documentation cleanup
   ([#605](https://github.com/Shopify/sarama/pull/605),
    [#621](https://github.com/Shopify/sarama/pull/621),
    [#654](https://github.com/Shopify/sarama/pull/654)).

Bug Fixes:
 - Fix race condition shutting down the OffsetManager
   ([#658](https://github.com/Shopify/sarama/pull/658)).

#### Version 1.8.0 (2016-02-01)

New Features:
 - Full support for Kafka 0.9:
   - All protocol messages and fields
   ([#586](https://github.com/Shopify/sarama/pull/586),
   [#588](https://github.com/Shopify/sarama/pull/588),
   [#590](https://github.com/Shopify/sarama/pull/590)).
   - Verified that TLS support works
   ([#581](https://github.com/Shopify/sarama/pull/581)).
   - Fixed the OffsetManager compatibility
   ([#585](https://github.com/Shopify/sarama/pull/585)).

Improvements:
 - Optimize for fewer system calls when reading from the network
   ([#584](https://github.com/Shopify/sarama/pull/584)).
 - Automatically retry `InvalidMessage` errors to match upstream behaviour
   ([#589](https://github.com/Shopify/sarama/pull/589)).

#### Version 1.7.0 (2015-12-11)

New Features:
 - Preliminary support for Kafka 0.9
   ([#572](https://github.com/Shopify/sarama/pull/572)). This comes with several
   caveats:
   - Protocol-layer support is mostly in place
     ([#577](https://github.com/Shopify/sarama/pull/577)), however Kafka 0.9
     renamed some messages and fields, which we did not in order to preserve API
     compatibility.
   - The producer and consumer work against 0.9, but the offset manager does
     not ([#573](https://github.com/Shopify/sarama/pull/573)).
   - TLS support may or may not work
     ([#581](https://github.com/Shopify/sarama/pull/581)).

Improvements:
 - Don't wait for request timeouts on dead brokers, greatly speeding recovery
   when the TCP connection is left hanging
   ([#548](https://github.com/Shopify/sarama/pull/548)).
 - Refactored part of the producer. The new version provides a much more elegant
   solution to [#449](https://github.com/Shopify/sarama/pull/449). It is also
   slightly more efficient, and much more precise in calculating batch sizes
   when compression is used
   ([#549](https://github.com/Shopify/sarama/pull/549),
   [#550](https://github.com/Shopify/sarama/pull/550),
   [#551](https://github.com/Shopify/sarama/pull/551)).

Bug Fixes:
 - Fix race condition in consumer test mock
   ([#553](https://github.com/Shopify/sarama/pull/553)).

#### Version 1.6.1 (2015-09-25)

Bug Fixes:
 - Fix panic that could occur if a user-supplied message value failed to encode
   ([#449](https://github.com/Shopify/sarama/pull/449)).

#### Version 1.6.0 (2015-09-04)

New Features:
 - Implementation of a consumer offset manager using the APIs introduced in
   Kafka 0.8.2. The API is designed mainly for integration into a future
   high-level consumer, not for direct use, although it is *possible* to use it
   directly.
   ([#461](https://github.com/Shopify/sarama/pull/461)).

Improvements:
 - CRC32 calculation is much faster on machines with SSE4.2 instructions,
   removing a major hotspot from most profiles
   ([#255](https://github.com/Shopify/sarama/pull/255)).

Bug Fixes:
 - Make protocol decoding more robust against some malformed packets generated
   by go-fuzz ([#523](https://github.com/Shopify/sarama/pull/523),
   [#525](https://github.com/Shopify/sarama/pull/525)) or found in other ways
   ([#528](https://github.com/Shopify/sarama/pull/528)).
 - Fix a potential race condition panic in the consumer on shutdown
   ([#529](https://github.com/Shopify/sarama/pull/529)).

#### Version 1.5.0 (2015-08-17)

New Features:
 - TLS-encrypted network connections are now supported. This feature is subject
   to change when Kafka releases built-in TLS support, but for now this is
   enough to work with TLS-terminating proxies
   ([#154](https://github.com/Shopify/sarama/pull/154)).

Improvements:
 - The consumer will not block if a single partition is not drained by the user;
   all other partitions will continue to consume normally
   ([#485](https://github.com/Shopify/sarama/pull/485)).
 - Formatting of error strings has been much improved
   ([#495](https://github.com/Shopify/sarama/pull/495)).
 - Internal refactoring of the producer for code cleanliness and to enable
   future work ([#300](https://github.com/Shopify/sarama/pull/300)).

Bug Fixes:
 - Fix a potential deadlock in the consumer on shutdown
   ([#475](https://github.com/Shopify/sarama/pull/475)).

#### Version 1.4.3 (2015-07-21)

Bug Fixes:
 - Don't include the partitioner in the producer's "fetch partitions"
   circuit-breaker ([#466](https://github.com/Shopify/sarama/pull/466)).
 - Don't retry messages until the broker is closed when abandoning a broker in
   the producer ([#468](https://github.com/Shopify/sarama/pull/468)).
 - Update the import path for snappy-go, it has moved again and the API has
   changed slightly ([#486](https://github.com/Shopify/sarama/pull/486)).

#### Version 1.4.2 (2015-05-27)

Bug Fixes:
 - Update the import path for snappy-go, it has moved from google code to github
   ([#456](https://github.com/Shopify/sarama/pull/456)).

#### Version 1.4.1 (2015-05-25)

Improvements:
 - Optimizations when decoding snappy messages, thanks to John Potocny
   ([#446](https://github.com/Shopify/sarama/pull/446)).

Bug Fixes:
 - Fix hypothetical race conditions on producer shutdown
   ([#450](https://github.com/Shopify/sarama/pull/450),
   [#451](https://github.com/Shopify/sarama/pull/451)).

#### Version 1.4.0 (2015-05-01)

New Features:
 - The consumer now implements `Topics()` and `Partitions()` methods to enable
   users to dynamically choose what topics/partitions to consume without
   instantiating a full client
   ([#431](https://github.com/Shopify/sarama/pull/431)).
 - The partition-consumer now exposes the high water mark offset value returned
   by the broker via the `HighWaterMarkOffset()` method ([#339](https://github.com/Shopify/sarama/pull/339)).
 - Added a `kafka-console-consumer` tool capable of handling multiple
   partitions, and deprecated the now-obsolete `kafka-console-partitionConsumer`
   ([#439](https://github.com/Shopify/sarama/pull/439),
   [#442](https://github.com/Shopify/sarama/pull/442)).

Improvements:
 - The producer's logging during retry scenarios is more consistent, more
   useful, and slightly less verbose
   ([#429](https://github.com/Shopify/sarama/pull/429)).
 - The client now shuffles its initial list of seed brokers in order to prevent
   thundering herd on the first broker in the list
   ([#441](https://github.com/Shopify/sarama/pull/441)).

Bug Fixes:
 - The producer now correctly manages its state if retries occur when it is
   shutting down, fixing several instances of confusing behaviour and at least
   one potential deadlock ([#419](https://github.com/Shopify/sarama/pull/419)).
 - The consumer now handles messages for different partitions asynchronously,
   making it much more resilient to specific user code ordering
   ([#325](https://github.com/Shopify/sarama/pull/325)).

#### Version 1.3.0 (2015-04-16)

New Features:
 - The client now tracks consumer group coordinators using
   ConsumerMetadataRequests similar to how it tracks partition leadership using
   regular MetadataRequests ([#411](https://github.com/Shopify/sarama/pull/411)).
   This adds two methods to the client API:
   - `Coordinator(consumerGroup string) (*Broker, error)`
   - `RefreshCoordinator(consumerGroup string) error`

Improvements:
 - ConsumerMetadataResponses now automatically create a Broker object out of the
   ID/address/port combination for the Coordinator; accessing the fields
   individually has been deprecated
   ([#413](https://github.com/Shopify/sarama/pull/413)).
 - Much improved handling of `OffsetOutOfRange` errors in the consumer.
   Consumers will fail to start if the provided offset is out of range
   ([#418](https://github.com/Shopify/sarama/pull/418))
   and they will automatically shut down if the offset falls out of range
   ([#424](https://github.com/Shopify/sarama/pull/424)).
 - Small performance improvement in encoding and decoding protocol messages
   ([#427](https://github.com/Shopify/sarama/pull/427)).

Bug Fixes:
 - Fix a rare race condition in the client's background metadata refresher if
   it happens to be activated while the client is being closed
   ([#422](https://github.com/Shopify/sarama/pull/422)).

#### Version 1.2.0 (2015-04-07)

Improvements:
 - The producer's behaviour when `Flush.Frequency` is set is now more intuitive
   ([#389](https://github.com/Shopify/sarama/pull/389)).
 - The producer is now somewhat more memory-efficient during and after retrying
   messages due to an improved queue implementation
   ([#396](https://github.com/Shopify/sarama/pull/396)).
 - The consumer produces much more useful logging output when leadership
   changes ([#385](https://github.com/Shopify/sarama/pull/385)).
 - The client's `GetOffset` method will now automatically refresh metadata and
   retry once in the event of stale information or similar
   ([#394](https://github.com/Shopify/sarama/pull/394)).
 - Broker connections now have support for using TCP keepalives
   ([#407](https://github.com/Shopify/sarama/issues/407)).

Bug Fixes:
 - The OffsetCommitRequest message now correctly implements all three possible
   API versions ([#390](https://github.com/Shopify/sarama/pull/390),
   [#400](https://github.com/Shopify/sarama/pull/400)).

#### Version 1.1.0 (2015-03-20)

Improvements:
 - Wrap the producer's partitioner call in a circuit-breaker so that repeatedly
   broken topics don't choke throughput
   ([#373](https://github.com/Shopify/sarama/pull/373)).

Bug Fixes:
 - Fix the producer's internal reference counting in certain unusual scenarios
   ([#367](https://github.com/Shopify/sarama/pull/367)).
 - Fix the consumer's internal reference counting in certain unusual scenarios
   ([#369](https://github.com/Shopify/sarama/pull/369)).
 - Fix a condition where the producer's internal control messages could have
   gotten stuck ([#368](https://github.com/Shopify/sarama/pull/368)).
 - Fix an issue where invalid partition lists would be cached when asking for
   metadata for a non-existant topic ([#372](https://github.com/Shopify/sarama/pull/372)).


#### Version 1.0.0 (2015-03-17)

Version 1.0.0 is the first tagged version, and is almost a complete rewrite. The primary differences with previous untagged versions are:

- The producer has been rewritten; there is now a `SyncProducer` with a blocking API, and an `AsyncProducer` that is non-blocking.
- The consumer has been rewritten to only open one connection per broker instead of one connection per partition.
- The main types of Sarama are now interfaces to make depedency injection easy; mock implementations for `Consumer`, `SyncProducer` and `AsyncProducer` are provided in the `github.com/Shopify/sarama/mocks` package.
- For most uses cases, it is no longer necessary to open a `Client`; this will be done for you.
- All the configuration values have been unified in the `Config` struct.
- Much improved test suite.
