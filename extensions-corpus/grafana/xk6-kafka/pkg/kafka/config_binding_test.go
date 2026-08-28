package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

// decodeJS decodes a JS object literal into dst through the standalone test
// host's field-name mapper, which resolves names through `js:` struct tags.
func decodeJS(t *testing.T, objectLiteral string, dst any) {
	t.Helper()
	rt := extensionapitest.NewVU().Runtime()
	v, err := rt.RunString("(" + objectLiteral + ")")
	require.NoError(t, err)
	require.NoError(t, rt.ExportTo(v, dst))
}

func TestConfigCamelCaseBinding(t *testing.T) {
	t.Parallel()

	var w WriterConfig
	decodeJS(t, `{brokers:["b"],topic:"t",autoCreateTopic:true,requiredAcks:1,batchBytes:1024,maxAttempts:3,writeTimeout:5}`, &w)
	require.True(t, w.AutoCreateTopic)
	require.NotNil(t, w.RequiredAcks)
	require.Equal(t, 1, *w.RequiredAcks)
	require.Equal(t, 1024, w.BatchBytes)
	require.NotNil(t, w.MaxAttempts)
	require.Equal(t, 3, *w.MaxAttempts)
	require.Equal(t, int64(5), w.WriteTimeout)

	var r ReaderConfig
	decodeJS(t, `{brokers:["b"],topic:"t",groupID:"g",groupTopics:["x","y"],minBytes:10,maxWait:"2s",startOffset:"start_offsets_last_offset",commitInterval:7}`, &r)
	require.Equal(t, "g", r.GroupID)
	require.Equal(t, []string{"x", "y"}, r.GroupTopics)
	require.Equal(t, 10, r.MinBytes)
	require.Equal(t, "2s", r.MaxWait)
	require.Equal(t, "start_offsets_last_offset", r.StartOffset)
	require.Equal(t, int64(7), r.CommitInterval)

	var c ConsumeConfig
	decodeJS(t, `{limit:5,expectTimeout:true,nanoPrecision:true}`, &c)
	require.Equal(t, 5, c.Limit)
	require.True(t, c.ExpectTimeout)
	require.True(t, c.NanoPrecision)

	var tc TopicConfig
	decodeJS(t, `{topic:"t",numPartitions:3,replicationFactor:2,configEntries:[{configName:"retention.ms",configValue:"1000"}]}`, &tc)
	require.Equal(t, 3, tc.NumPartitions)
	require.Equal(t, 2, tc.ReplicationFactor)
	require.Len(t, tc.ConfigEntries, 1)
	require.Equal(t, "retention.ms", tc.ConfigEntries[0].ConfigName)
	require.Equal(t, "1000", tc.ConfigEntries[0].ConfigValue)

	var tls TLSConfig
	decodeJS(t, `{enableTls:true,insecureSkipTlsVerify:true,minVersion:"tlsv1.2",serverCaPem:"PEM"}`, &tls)
	require.True(t, tls.EnableTLS)
	require.True(t, tls.InsecureSkipTLSVerify)
	require.Equal(t, "tlsv1.2", tls.MinVersion)
	require.Equal(t, "PEM", tls.ServerCaPem)

	var conn ConnectionConfig
	decodeJS(t, `{address:"a:9092",sasl:{username:"u",algorithm:"sasl_plain",awsProfile:"p"},tls:{enableTls:true}}`, &conn)
	require.Equal(t, "a:9092", conn.Address)
	require.NotNil(t, conn.SASL)
	require.Equal(t, "u", conn.SASL.Username)
	require.Equal(t, "p", conn.SASL.AWSProfile)
	require.NotNil(t, conn.TLS)
	require.True(t, conn.TLS.EnableTLS)

	var jks JKSConfig
	decodeJS(t, `{path:"/k.jks",clientKeyAlias:"ck",serverCaAlias:"ca"}`, &jks)
	require.Equal(t, "ck", jks.ClientKeyAlias)
	require.Equal(t, "ca", jks.ServerCaAlias)
}

func TestConsumedMessageOutputIsCamelCase(t *testing.T) {
	t.Parallel()
	rt := extensionapitest.NewVU().Runtime()
	require.NoError(t, rt.Set("m", ConsumedMessage{Topic: "t", HighWaterMark: 42}))
	v, err := rt.RunString(`m.highWaterMark === 42 && m.topic === "t" && m.high_water_mark === undefined`)
	require.NoError(t, err)
	require.True(t, v.ToBoolean(), "consume() must expose camelCase highWaterMark, not high_water_mark")
}
