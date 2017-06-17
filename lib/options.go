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
	// From https://golang.org/pkg/crypto/tls/#pkg-constants
	versionMap := map[string]int{
		"ssl3.0": tls.VersionSSL30,
		"tls1.0": tls.VersionTLS10,
		"tls1.1": tls.VersionTLS11,
		"tls1.2": tls.VersionTLS12,
	}

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

	var minVersion int
	var maxVersion int
	var ok bool
	if minVersion, ok = versionMap[fields.Min]; !ok {
		return errors.New("Unknown TLS version : " + fields.Min)
	}

	if maxVersion, ok = versionMap[fields.Max]; !ok {
		return errors.New("Unknown TLS version : " + fields.Max)
	}

	v.Min = minVersion
	v.Max = maxVersion

	return nil
}

type TLSCipherSuites struct {
	Values []uint16
}

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

	s.Values = suiteIDs

	return nil
}

type Options struct {
	Paused     null.Bool    `json:"paused"`
	VUs        null.Int     `json:"vus"`
	VUsMax     null.Int     `json:"vusMax"`
	Duration   NullDuration `json:"duration"`
	Iterations null.Int     `json:"iterations"`
	Stages     []Stage      `json:"stages"`

	Linger        null.Bool `json:"linger"`
	NoUsageReport null.Bool `json:"noUsageReport"`

	MaxRedirects          null.Int         `json:"maxRedirects"`
	InsecureSkipTLSVerify null.Bool        `json:"insecureSkipTLSVerify"`
	TLSCipherSuites       *TLSCipherSuites `json:"tlsCipherSuites"`
	TLSVersion            *TLSVersion      `json:"tlsVersion"`
	NoConnectionReuse     null.Bool        `json:"noConnectionReuse"`
	UserAgent             null.String      `json:"userAgent"`
	Throw                 null.Bool        `json:"throw"`

	Thresholds map[string]stats.Thresholds `json:"thresholds"`

	// These values are for third party collectors' benefit.
	External map[string]interface{} `json:"ext"`
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
	if opts.Linger.Valid {
		o.Linger = opts.Linger
	}
	if opts.NoUsageReport.Valid {
		o.NoUsageReport = opts.NoUsageReport
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
