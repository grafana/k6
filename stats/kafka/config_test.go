/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package kafka

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/stats/influxdb"
)

func TestConfigParseArg(t *testing.T) {
	c, err := ParseArg("brokers=broker1,topic=someTopic,format=influxdb")
	expInfluxConfig := influxdb.Config{}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker1"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic2,format=json")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic2"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=influxdb,influxdb.tagsAsFields=fake")
	expInfluxConfig = influxdb.Config{
		TagsAsFields: []string{"fake"},
	}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker2,broker3:9092},topic=someTopic,format=influxdb,influxdb.tagsAsFields={fake,anotherFake}")
	expInfluxConfig = influxdb.Config{
		TagsAsFields: []string{"fake", "anotherFake"},
	}
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker2", "broker3:9092"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("influxdb"), c.Format)
	assert.Equal(t, expInfluxConfig, c.InfluxDBConfig)

	c, err = ParseArg("brokers={broker-2.kafka.com:9093,broker-3.kafka.com:9093},topic=someTopic,format=json,tls_security=true,certificate=cert.pem,private_key=key.pem,certificate_authority=ca.pem,insecure_skip=true")
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker-2.kafka.com:9093", "broker-3.kafka.com:9093"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)
	assert.Equal(t, true, c.TLSSecurity)
	assert.Equal(t, null.StringFrom("cert.pem"), c.Certificate)
	assert.Equal(t, null.StringFrom("key.pem"), c.PrivateKey)
	assert.Equal(t, null.StringFrom("ca.pem"), c.CertificateAuthority)
	assert.Equal(t, true, c.InsecureSkip)

	// Test with security layer

	cert, err := os.Create("cert.pem")
	assert.Nil(t, err)
	_, err = cert.WriteString("cert")
	assert.Nil(t, err)
	err = cert.Close()
	assert.Nil(t, err)

	key, err := os.Create("key.pem")
	assert.Nil(t, err)
	_, err = key.WriteString("private_key")
	assert.Nil(t, err)
	err = key.Close()
	assert.Nil(t, err)

	ca, err := os.Create("ca.pem")
	assert.Nil(t, err)
	_, err = ca.WriteString("certificate_authority")
	assert.Nil(t, err)
	err = ca.Close()
	assert.Nil(t, err)

	defer func() {
		err = os.Remove("cert.pem")
		assert.Nil(t, err)
		err = os.Remove("key.pem")
		assert.Nil(t, err)
		err = os.Remove("ca.pem")
		assert.Nil(t, err)
	}()

	// Get working directory
	wd, err := os.Getwd()
	assert.Nil(t, err)

	c, err = ParseTLSSecurity(c)
	assert.Nil(t, err)
	assert.Equal(t, []string{"broker-2.kafka.com:9093", "broker-3.kafka.com:9093"}, c.Brokers)
	assert.Equal(t, null.StringFrom("someTopic"), c.Topic)
	assert.Equal(t, null.StringFrom("json"), c.Format)
	assert.Equal(t, true, c.TLSSecurity)
	assert.Equal(t, null.StringFrom(filepath.Join(wd, "cert.pem")), c.Certificate)
	assert.Equal(t, null.StringFrom(filepath.Join(wd, "key.pem")), c.PrivateKey)
	assert.Equal(t, null.StringFrom(filepath.Join(wd, "ca.pem")), c.CertificateAuthority)
	assert.Equal(t, true, c.InsecureSkip)
}
