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

package influxdb

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
