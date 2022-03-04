/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package cloud

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"

	easyjson "github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/cloudapi"
)

// MetricsClient is a wrapper around the cloudapi.Client that is also capable of pushing
type MetricsClient struct {
	*cloudapi.Client

	pushBufferPool sync.Pool
	logger         logrus.FieldLogger
	host           string
	noCompress     bool
}

// NewMetricsClient creates and initializes a new MetricsClient.
func NewMetricsClient(client *cloudapi.Client, logger logrus.FieldLogger, host string, noCompress bool) *MetricsClient {
	return &MetricsClient{
		Client:     client,
		logger:     logger,
		host:       host,
		noCompress: noCompress,
		pushBufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

// PushMetric pushes the provided metric samples for the given referenceID
func (mc *MetricsClient) PushMetric(referenceID string, s []*Sample) error {
	start := time.Now()
	url := fmt.Sprintf("%s/v1/metrics/%s", mc.host, referenceID)

	jsonStart := time.Now()
	b, err := easyjson.Marshal(samples(s))
	if err != nil {
		return err
	}
	jsonTime := time.Since(jsonStart)

	// TODO: change the context, maybe to one with a timeout
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Payload-Sample-Count", strconv.Itoa(len(s)))
	var additionalFields logrus.Fields

	if !mc.noCompress {
		buf := mc.pushBufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer mc.pushBufferPool.Put(buf)
		unzippedSize := len(b)
		buf.Grow(unzippedSize / expectedGzipRatio)
		gzipStart := time.Now()
		{
			g, _ := gzip.NewWriterLevel(buf, gzip.BestSpeed)
			if _, err = g.Write(b); err != nil {
				return err
			}
			if err = g.Close(); err != nil {
				return err
			}
		}
		gzipTime := time.Since(gzipStart)

		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("X-Payload-Byte-Count", strconv.Itoa(unzippedSize))

		additionalFields = logrus.Fields{
			"unzipped_size":  unzippedSize,
			"gzip_t":         gzipTime,
			"content_length": buf.Len(),
		}

		b = buf.Bytes()
	}

	req.Header.Set("Content-Length", strconv.Itoa(len(b)))
	req.Body = ioutil.NopCloser(bytes.NewReader(b))
	req.GetBody = func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(b)), nil
	}

	err = mc.Client.Do(req, nil)

	mc.logger.WithFields(logrus.Fields{
		"t":         time.Since(start),
		"json_t":    jsonTime,
		"part_size": len(s),
	}).WithFields(additionalFields).Debug("Pushed part to cloud")

	return err
}
