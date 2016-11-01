package influxdb

import (
	"errors"
	"github.com/influxdata/influxdb/client/v2"
	"net/url"
	"strconv"
	"time"
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
