package influxdb

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
	"time"
)

func TestMakeUDPConfigFromURL(t *testing.T) {
	u, err := url.Parse("udp://1.2.3.4:12345")
	assert.NoError(t, err)

	conf, err := makeUDPConfigFromURL(u)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4:12345", conf.Addr)
	assert.Equal(t, 0, conf.PayloadSize)
}

func TestMakeUDPConfigFromURLWithPayloadSize(t *testing.T) {
	u, err := url.Parse("udp://1.2.3.4:12345?payload_size=512")
	assert.NoError(t, err)

	conf, err := makeUDPConfigFromURL(u)
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4:12345", conf.Addr)
	assert.Equal(t, 512, conf.PayloadSize)
}

func TestMakeHTTPConfigFromURL(t *testing.T) {
	u, err := url.Parse("http://1.2.3.4:12345")
	assert.NoError(t, err)

	conf, err := makeHTTPConfigFromURL(u)
	assert.NoError(t, err)
	assert.Equal(t, "http://1.2.3.4:12345", conf.Addr)
	assert.Equal(t, "", conf.Username)
	assert.Equal(t, "", conf.Password)
	assert.Equal(t, 0*time.Second, conf.Timeout)
	assert.Equal(t, false, conf.InsecureSkipVerify)
}

func TestMakeHTTPConfigFromURLInsecureHTTPS(t *testing.T) {
	u, err := url.Parse("https://1.2.3.4:12345?insecure_skip_verify=true")
	assert.NoError(t, err)

	conf, err := makeHTTPConfigFromURL(u)
	assert.NoError(t, err)
	assert.Equal(t, "https://1.2.3.4:12345", conf.Addr)
	assert.Equal(t, "", conf.Username)
	assert.Equal(t, "", conf.Password)
	assert.Equal(t, 0*time.Second, conf.Timeout)
	assert.Equal(t, true, conf.InsecureSkipVerify)
}

func TestMakeBatchConfigFromURL(t *testing.T) {
	u, err := url.Parse("http://1.2.3.4:12345/database?precision=s&retention_policy=policy1&write_consistency=2")
	assert.NoError(t, err)

	conf, err := makeBatchConfigFromURL(u)
	assert.NoError(t, err)
	assert.Equal(t, "database", conf.Database)
	assert.Equal(t, "s", conf.Precision)
	assert.Equal(t, "policy1", conf.RetentionPolicy)
	assert.Equal(t, "2", conf.WriteConsistency)
}
