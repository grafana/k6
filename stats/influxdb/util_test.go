package influxdb

import (
	"github.com/loadimpact/speedboat/stats"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestParseURL(t *testing.T) {
	conf, db, err := parseURL("http://username:password@hostname.local:8086/db")
	assert.Nil(t, err, "couldn't parse URL")
	assert.Equal(t, "username", conf.Username, "incorrect username")
	assert.Equal(t, "password", conf.Password, "incorrect password")
	assert.Equal(t, "http://hostname.local:8086", conf.Addr, "incorrect address")
	assert.Equal(t, "db", db, "incorrect db")
}

func TestParseURLNoAuth(t *testing.T) {
	conf, db, err := parseURL("http://hostname.local:8086/db")
	assert.Nil(t, err, "couldn't parse URL")
	assert.Equal(t, "http://hostname.local:8086", conf.Addr, "incorrect address")
	assert.Equal(t, "db", db, "incorrect db")
}

func TestParseURLNoDB(t *testing.T) {
	_, _, err := parseURL("http://hostname.local:8086")
	assert.NotNil(t, err, "no error reported")
}

func TestMakeInfluxPoint(t *testing.T) {
	now := time.Now()
	pt, err := makeInfluxPoint(stats.Point{
		Stat:   &stats.Stat{Name: "test"},
		Time:   now,
		Tags:   stats.Tags{"a": "b"},
		Values: stats.Values{"value": 12345},
	})
	assert.NoError(t, err)
	assert.Equal(t, "test", pt.Name())
	assert.Equal(t, now, pt.Time())
	assert.EqualValues(t, map[string]string{"a": "b"}, pt.Tags())
	assert.EqualValues(t, map[string]interface{}{"value": float64(12345)}, pt.Fields())
}
