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
	"fmt"
	"time"

	"github.com/kubernetes/helm/pkg/strvals"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats/influxdb"
)

// Config is the config for the kafka collector
type Config struct {
	// Connection.
	Brokers []string `json:"brokers" envconfig:"K6_KAFKA_BROKERS"`

	// Samples.
	Topic        null.String        `json:"topic" envconfig:"K6_KAFKA_TOPIC"`
	Format       null.String        `json:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval types.NullDuration `json:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`

	InfluxDBConfig influxdb.Config `json:"influxdb"`

	// TLS
	TLSSecurity          bool        `json:"tls_security" envconfig:"K6_KAFKA_TLS_SECURITY"`
	Certificate          null.String `json:"certificate" envconfig:"K6_KAFKA_CERTIFICATE"`
	PrivateKey           null.String `json:"private_key" envconfig:"K6_KAFKA_PRIVATE_KEY"`
	CertificateAuthority null.String `json:"certificate_authority" envconfig:"K6_KAFKA_CERTIFICATE_AUTHORITY"`
	InsecureSkipVerify   bool        `json:"insecure_skip_verify" envconfig:"K6_KAFKA_INSECURE_SKIP_VERIFY"`
}

// config is a duplicate of ConfigFields as we can not mapstructure.Decode into
// null types so we duplicate the struct with primitive types to Decode into
type config struct {
	Brokers      []string `json:"brokers" mapstructure:"brokers" envconfig:"K6_KAFKA_BROKERS"`
	Topic        string   `json:"topic" mapstructure:"topic" envconfig:"K6_KAFKA_TOPIC"`
	Format       string   `json:"format" mapstructure:"format" envconfig:"K6_KAFKA_FORMAT"`
	PushInterval string   `json:"push_interval" mapstructure:"push_interval" envconfig:"K6_KAFKA_PUSH_INTERVAL"`

	InfluxDBConfig influxdb.Config `json:"influxdb" mapstructure:"influxdb"`

	TLSSecurity          bool   `json:"tls_security" mapstructure:"tls_security" envconfig:"K6_KAFKA_TLS_SECURITY"`
	Certificate          string `json:"certificate" mapstructure:"certificate" envconfig:"K6_KAFKA_CERTIFICATE"`
	PrivateKey           string `json:"private_key" mapstructure:"private_key" envconfig:"K6_KAFKA_PRIVATE_KEY"`
	CertificateAuthority string `json:"certificate_authority" mapstructure:"certificate_authority" envconfig:"K6_KAFKA_CERTIFICATE_AUTHORITY"`
	InsecureSkipVerify   bool   `json:"insecure_skip_verify" mapstructure:"insecure_skip_verify" envconfig:"K6_KAFKA_INSECURE_SKIP_VERIFY"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Format:       null.StringFrom("json"),
		PushInterval: types.NullDurationFrom(1 * time.Second),
	}
}

// Apply config
func (c Config) Apply(cfg Config) Config {
	if len(cfg.Brokers) > 0 {
		c.Brokers = cfg.Brokers
	}
	if cfg.Format.Valid {
		c.Format = cfg.Format
	}
	if cfg.Topic.Valid {
		c.Topic = cfg.Topic
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.TLSSecurity {
		c.TLSSecurity = cfg.TLSSecurity
	}
	if cfg.Certificate.Valid {
		c.Certificate = cfg.Certificate
	}
	if cfg.PrivateKey.Valid {
		c.PrivateKey = cfg.PrivateKey
	}
	if cfg.CertificateAuthority.Valid {
		c.CertificateAuthority = cfg.CertificateAuthority
	}
	if cfg.InsecureSkipVerify {
		c.InsecureSkipVerify = cfg.InsecureSkipVerify
	}
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}
	params, err := strvals.Parse(arg)

	if err != nil {
		return c, err
	}

	if v, ok := params["brokers"].(string); ok {
		params["brokers"] = []string{v}
	}

	if v, ok := params["influxdb"].(map[string]interface{}); ok {
		influxConfig, err := influxdb.ParseMap(v)
		if err != nil {
			return c, err
		}
		c.InfluxDBConfig = influxConfig
	}
	delete(params, "influxdb")

	if v, ok := params["push_interval"].(string); ok {
		err := c.PushInterval.UnmarshalText([]byte(v))
		if err != nil {
			return c, err
		}
	}

	var cfg config
	err = mapstructure.Decode(params, &cfg)
	if err != nil {
		return c, err
	}

	c.Brokers = cfg.Brokers
	c.Topic = null.StringFrom(cfg.Topic)
	c.Format = null.StringFrom(cfg.Format)
	c.TLSSecurity = cfg.TLSSecurity
	c.Certificate = null.StringFrom(cfg.Certificate)
	c.PrivateKey = null.StringFrom(cfg.PrivateKey)
	c.CertificateAuthority = null.StringFrom(cfg.CertificateAuthority)
	c.InsecureSkipVerify = cfg.InsecureSkipVerify

	return c, nil
}

//ParseTLSSecurity validate and read tls security config
func ParseTLSSecurity(c Config) (Config, error) {
	if c.Certificate.String == "" || c.PrivateKey.String == "" {
		return c, fmt.Errorf("missing certificate and private key")
	}

	//Read certificate
	cPath, err := GetAbsolutelyFilePath(c.Certificate.String)
	if err != nil {
		return c, err
	}

	c.Certificate = null.StringFrom(string(cPath))

	//Read private key
	pkPath, err := GetAbsolutelyFilePath(c.PrivateKey.String)
	if err != nil {
		return c, err
	}

	c.PrivateKey = null.StringFrom(string(pkPath))

	if c.CertificateAuthority.String != "" {
		//Read certificate authority
		caPath, err := GetAbsolutelyFilePath(c.CertificateAuthority.String)
		if err != nil {
			return c, err
		}

		c.CertificateAuthority = null.StringFrom(caPath)
	}

	return c, nil
}
