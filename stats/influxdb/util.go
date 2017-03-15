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
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/influxdata/influxdb/client/v2"
)

var (
	ErrNoDatabase = errors.New("influxdb output: no database specified")
)

func parseURL(u *url.URL) (client.Client, client.BatchPointsConfig, error) {
	batchConf, err := makeBatchConfigFromURL(u)
	if err != nil {
		return nil, client.BatchPointsConfig{}, err
	}

	if u.Scheme == "udp" {
		conf, err := makeUDPConfigFromURL(u)
		if err != nil {
			return nil, batchConf, err
		}
		c, err := client.NewUDPClient(conf)
		if err != nil {
			return nil, batchConf, err
		}
		return c, batchConf, nil
	}

	conf, err := makeHTTPConfigFromURL(u)
	if err != nil {
		return nil, batchConf, err
	}
	c, err := client.NewHTTPClient(conf)
	if err != nil {
		return nil, batchConf, err
	}

	// Create database if it does not exist
	q := client.NewQuery("CREATE DATABASE "+batchConf.Database, "", "")
	_, err = c.Query(q)
	if err != nil {
		return nil, batchConf, err
	}

	return c, batchConf, nil
}

func makeUDPConfigFromURL(u *url.URL) (client.UDPConfig, error) {
	payloadSize := 0
	payloadSizeS := u.Query().Get("payload_size")
	if payloadSizeS != "" {
		s, err := strconv.ParseInt(payloadSizeS, 10, 32)
		if err != nil {
			return client.UDPConfig{}, err
		}
		payloadSize = int(s)
	}

	return client.UDPConfig{
		Addr:        u.Host,
		PayloadSize: payloadSize,
	}, nil
}

func makeHTTPConfigFromURL(u *url.URL) (client.HTTPConfig, error) {
	q := u.Query()

	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	timeout := 0 * time.Second
	if ts := q.Get("timeout"); ts != "" {
		t, err := time.ParseDuration(ts)
		if err != nil {
			return client.HTTPConfig{}, err
		}
		timeout = t
	}
	insecureSkipVerify := q.Get("insecure_skip_verify") != ""

	return client.HTTPConfig{
		Addr:               u.Scheme + "://" + u.Host,
		Username:           username,
		Password:           password,
		Timeout:            timeout,
		InsecureSkipVerify: insecureSkipVerify,
	}, nil
}

func makeBatchConfigFromURL(u *url.URL) (client.BatchPointsConfig, error) {
	if u.Path == "" || u.Path == "/" {
		return client.BatchPointsConfig{}, ErrNoDatabase
	}

	q := u.Query()
	return client.BatchPointsConfig{
		Database:         u.Path[1:], // strip leading "/"
		Precision:        q.Get("precision"),
		RetentionPolicy:  q.Get("retention_policy"),
		WriteConsistency: q.Get("write_consistency"),
	}, nil
}
