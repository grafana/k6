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

package lib

import (
	"crypto/tls"
	"encoding/json"
	"errors"

	"github.com/loadimpact/k6/stats"
	"gopkg.in/guregu/null.v3"
)

type TLSVersion struct {
	Min int
	Max int
}

func (v *TLSVersion) UnmarshalJSON(data []byte) error {
	version := TLSVersion{}

	// Version might be a string or an object with separate min & max fields
	var fields struct {
		Min string `json:"min"`
		Max string `json:"max"`
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		switch err.(type) {
		case *json.UnmarshalTypeError:
			// Check if it's a type error and the user has passed a string
			var version string
			if otherErr := json.Unmarshal(data, &version); otherErr != nil {
				switch otherErr.(type) {
				case *json.UnmarshalTypeError:
					return errors.New("Type error: the value of tlsVersion " +
						"should be an object with min/max fields or a string")
				}

				// Some other error occurred
				return otherErr
			}
			// It was a string, assign it to both min & max
			fields.Min = version
			fields.Max = version
		default:
			return err
		}
	}

	var ok bool
	if version.Min, ok = SupportedTLSVersions[fields.Min]; !ok {
		return errors.New("Unknown TLS version : " + fields.Min)
	}

	if version.Max, ok = SupportedTLSVersions[fields.Max]; !ok {
		return errors.New("Unknown TLS version : " + fields.Max)
	}

	*v = version

	return nil
}

type TLSCipherSuites []uint16

func (s *TLSCipherSuites) UnmarshalJSON(data []byte) error {
	var suiteNames []string
	if err := json.Unmarshal(data, &suiteNames); err != nil {
		return err
	}

	var suiteIDs []uint16
	for _, name := range suiteNames {
		if suiteID, ok := SupportedTLSCipherSuites[name]; ok {
			suiteIDs = append(suiteIDs, suiteID)
		} else {
			return errors.New("Unknown cipher suite: " + name)
		}
	}

	*s = suiteIDs

	return nil
}

type TLSAuthFields struct {
	Cert    string   `json:"cert"`
	Key     string   `json:"key"`
	Domains []string `json:"domains"`
}

type TLSAuth struct {
	TLSAuthFields
	certificate *tls.Certificate
}

func (c *TLSAuth) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &c.TLSAuthFields); err != nil {
		return err
	}
	if _, err := c.Certificate(); err != nil {
		return err
	}
	return nil
}

func (c *TLSAuth) Certificate() (*tls.Certificate, error) {
	if c.certificate == nil {
		cert, err := tls.X509KeyPair([]byte(c.Cert), []byte(c.Key))
		if err != nil {
			return nil, err
		}
		c.certificate = &cert
	}
	return c.certificate, nil
}

type Options struct {
	Paused     null.Bool    `json:"paused" envconfig:"paused"`
	VUs        null.Int     `json:"vus" envconfig:"vus"`
	VUsMax     null.Int     `json:"vusMax" envconfig:"vus_max"`
	Duration   NullDuration `json:"duration" envconfig:"duration"`
	Iterations null.Int     `json:"iterations" envconfig:"iterations"`
	Stages     []Stage      `json:"stages" envconfig:"stages"`

	MaxRedirects          null.Int         `json:"maxRedirects" envconfig:"max_redirects"`
	InsecureSkipTLSVerify null.Bool        `json:"insecureSkipTLSVerify" envconfig:"insecure_skip_tls_verify"`
	TLSCipherSuites       *TLSCipherSuites `json:"tlsCipherSuites" envconfig:"tls_cipher_suites"`
	TLSVersion            *TLSVersion      `json:"tlsVersion" envconfig:"tls_version"`
	TLSAuth               []*TLSAuth       `json:"tlsAuth" envconfig:"tlsauth"`
	NoConnectionReuse     null.Bool        `json:"noConnectionReuse" envconfig:"no_connection_reuse"`
	UserAgent             null.String      `json:"userAgent" envconfig:"user_agent"`
	Throw                 null.Bool        `json:"throw" envconfig:"throw"`

	Thresholds map[string]stats.Thresholds `json:"thresholds" envconfig:"thresholds"`

	// These values are for third party collectors' benefit.
	// Can't be set through env vars.
	External map[string]interface{} `json:"ext" ignored:"true"`
}

func (o Options) Apply(opts Options) Options {
	if opts.Paused.Valid {
		o.Paused = opts.Paused
	}
	if opts.VUs.Valid {
		o.VUs = opts.VUs
	}
	if opts.VUsMax.Valid {
		o.VUsMax = opts.VUsMax
	}
	if opts.Duration.Valid {
		o.Duration = opts.Duration
	}
	if opts.Iterations.Valid {
		o.Iterations = opts.Iterations
	}
	if opts.Stages != nil {
		o.Stages = opts.Stages
	}
	if opts.MaxRedirects.Valid {
		o.MaxRedirects = opts.MaxRedirects
	}
	if opts.InsecureSkipTLSVerify.Valid {
		o.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
	}
	if opts.TLSCipherSuites != nil {
		o.TLSCipherSuites = opts.TLSCipherSuites
	}
	if opts.TLSVersion != nil {
		o.TLSVersion = opts.TLSVersion
	}
	if opts.TLSAuth != nil {
		o.TLSAuth = opts.TLSAuth
	}
	if opts.NoConnectionReuse.Valid {
		o.NoConnectionReuse = opts.NoConnectionReuse
	}
	if opts.UserAgent.Valid {
		o.UserAgent = opts.UserAgent
	}
	if opts.Throw.Valid {
		o.Throw = opts.Throw
	}
	if opts.Thresholds != nil {
		o.Thresholds = opts.Thresholds
	}
	if opts.External != nil {
		o.External = opts.External
	}
	return o
}
