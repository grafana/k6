package kafka

import "time"

// Constant values exported by the k6/x/kafka module. These mirror the flat
// top-level constants declared in index.d.ts exactly; see that file for the
// authoritative contract. They are exported as individual constants, not as
// enum objects.
const (
	// Compression codecs.
	codecGzip   = "gzip"
	codecSnappy = "snappy"
	codecLz4    = "lz4"
	codecZstd   = "zstd"

	// SASL mechanisms.
	saslNone        = "none"
	saslPlain       = "sasl_plain"
	saslScramSha256 = "sasl_scram_sha256"
	saslScramSha512 = "sasl_scram_sha512"
	saslSsl         = "sasl_ssl"
	saslAwsIam      = "sasl_aws_iam"

	// TLS versions.
	tls10 = "tlsv1.0"
	tls11 = "tlsv1.1"
	tls12 = "tlsv1.2"
	tls13 = "tlsv1.3"

	// Element types.
	elementKey   = "key"
	elementValue = "value"

	// Isolation levels.
	isolationReadUncommitted = "isolation_level_read_uncommitted"
	isolationReadCommitted   = "isolation_level_read_committed"

	// Start offsets.
	startOffsetFirst = "start_offsets_first_offset"
	startOffsetLast  = "start_offsets_last_offset"

	// Subject name strategies.
	topicNameStrategy       = "TopicNameStrategy"
	recordNameStrategy      = "RecordNameStrategy"
	topicRecordNameStrategy = "TopicRecordNameStrategy"

	// Balancers.
	balancerRoundRobin = "balancer_roundrobin"
	balancerLeastBytes = "balancer_leastbytes"
	balancerHash       = "balancer_hash"
	balancerCrc32      = "balancer_crc32"
	balancerMurmur2    = "balancer_murmur2"

	// Group balancers.
	groupBalancerRange        = "group_balancer_range"
	groupBalancerRoundRobin   = "group_balancer_round_robin"
	groupBalancerRackAffinity = "group_balancer_rack_affinity"

	// Schema types.
	schemaTypeString   = "STRING"
	schemaTypeBytes    = "BYTES"
	schemaTypeAvro     = "AVRO"
	schemaTypeJSON     = "JSON"
	schemaTypeProtobuf = "PROTOBUF"
)

// moduleConstants returns the flat name→value map exported by the module.
// The values match index.d.ts; TIME units are nanosecond counts.
func moduleConstants() map[string]any {
	return map[string]any{
		// Compression codecs.
		"CODEC_GZIP":   codecGzip,
		"CODEC_SNAPPY": codecSnappy,
		"CODEC_LZ4":    codecLz4,
		"CODEC_ZSTD":   codecZstd,

		// SASL mechanisms.
		"NONE":              saslNone,
		"SASL_PLAIN":        saslPlain,
		"SASL_SCRAM_SHA256": saslScramSha256,
		"SASL_SCRAM_SHA512": saslScramSha512,
		"SASL_SSL":          saslSsl,
		"SASL_AWS_IAM":      saslAwsIam,

		// TLS versions.
		"TLS_1_0": tls10,
		"TLS_1_1": tls11,
		"TLS_1_2": tls12,
		"TLS_1_3": tls13,

		// Element types.
		"KEY":   elementKey,
		"VALUE": elementValue,

		// Isolation levels.
		"ISOLATION_LEVEL_READ_UNCOMMITTED": isolationReadUncommitted,
		"ISOLATION_LEVEL_READ_COMMITTED":   isolationReadCommitted,

		// Start offsets (new + backward-compatibility aliases).
		"START_OFFSETS_FIRST_OFFSET": startOffsetFirst,
		"START_OFFSETS_LAST_OFFSET":  startOffsetLast,
		"FIRST_OFFSET":               startOffsetFirst,
		"LAST_OFFSET":                startOffsetLast,

		// Subject name strategies.
		"TOPIC_NAME_STRATEGY":        topicNameStrategy,
		"RECORD_NAME_STRATEGY":       recordNameStrategy,
		"TOPIC_RECORD_NAME_STRATEGY": topicRecordNameStrategy,

		// Balancers.
		"BALANCER_ROUND_ROBIN": balancerRoundRobin,
		"BALANCER_LEAST_BYTES": balancerLeastBytes,
		"BALANCER_HASH":        balancerHash,
		"BALANCER_CRC32":       balancerCrc32,
		"BALANCER_MURMUR2":     balancerMurmur2,

		// Group balancers.
		"GROUP_BALANCER_RANGE":         groupBalancerRange,
		"GROUP_BALANCER_ROUND_ROBIN":   groupBalancerRoundRobin,
		"GROUP_BALANCER_RACK_AFFINITY": groupBalancerRackAffinity,

		// Schema types.
		"SCHEMA_TYPE_STRING":   schemaTypeString,
		"SCHEMA_TYPE_BYTES":    schemaTypeBytes,
		"SCHEMA_TYPE_AVRO":     schemaTypeAvro,
		"SCHEMA_TYPE_JSON":     schemaTypeJSON,
		"SCHEMA_TYPE_PROTOBUF": schemaTypeProtobuf,

		// Time units, expressed in nanoseconds.
		"NANOSECOND":  int64(time.Nanosecond),
		"MICROSECOND": int64(time.Microsecond),
		"MILLISECOND": int64(time.Millisecond),
		"SECOND":      int64(time.Second),
		"MINUTE":      int64(time.Minute),
		"HOUR":        int64(time.Hour),
	}
}
