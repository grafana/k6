/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
)

//easyjson:json
type msg struct {
	Streams        []msgStreams        `json:"streams"`
	DroppedEntries []msgDroppedEntries `json:"dropped_entries"`
}

//easyjson:json
type msgStreams struct {
	Stream map[string]string `json:"stream"`
	Values [][2]string       `json:"values"` // this can be optimized
}

//easyjson:json
type msgDroppedEntries struct {
	Labels    map[string]string `json:"labels"`
	Timestamp string            `json:"timestamp"`
}

func (m *msg) Log(logger logrus.FieldLogger) {
	var level string

	for _, stream := range m.Streams {
		fields := labelsToLogrusFields(stream.Stream)
		var ok bool
		if level, ok = stream.Stream["level"]; ok {
			delete(fields, "level")
		}

		for _, value := range stream.Values {
			nsec, _ := strconv.Atoi(value[0])
			e := logger.WithFields(fields).WithTime(time.Unix(0, int64(nsec)))
			lvl, err := logrus.ParseLevel(level)
			if err != nil {
				e.Info(value[1])
				e.Warn("last message had unknown level " + level)
			} else {
				e.Log(lvl, value[1])
			}
		}
	}

	for _, dropped := range m.DroppedEntries {
		nsec, _ := strconv.Atoi(dropped.Timestamp)
		logger.WithFields(labelsToLogrusFields(dropped.Labels)).WithTime(time.Unix(0, int64(nsec))).Warn("dropped")
	}
}

func labelsToLogrusFields(labels map[string]string) logrus.Fields {
	fields := make(logrus.Fields, len(labels))

	for key, val := range labels {
		fields[key] = val
	}

	return fields
}

func (c *Config) getRequest(referenceID string, start time.Duration) (*url.URL, error) {
	u, err := url.Parse(c.LogsTailURL.String)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse cloud logs host %w", err)
	}

	u.RawQuery = fmt.Sprintf(`query={test_run_id="%s"}&start=%d`,
		referenceID,
		time.Now().Add(-start).UnixNano(),
	)

	return u, nil
}

// StreamLogsToLogger streams the logs for the configured test to the provided logger until ctx is
// Done or an error occurs.
func (c *Config) StreamLogsToLogger(
	ctx context.Context, logger logrus.FieldLogger, referenceID string, start time.Duration,
) error {
	u, err := c.getRequest(referenceID, start)
	if err != nil {
		return err
	}

	headers := make(http.Header)
	headers.Add("Sec-WebSocket-Protocol", "token="+c.Token.String)

	// We don't need to close the http body or use it for anything until we want to actually log
	// what the server returned as body when it errors out
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), headers) //nolint:bodyclose
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()

		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "closing"),
			time.Now().Add(time.Second))

		_ = conn.Close()
	}()

	msgBuffer := make(chan []byte, 10)

	defer close(msgBuffer)

	go func() {
		for message := range msgBuffer {
			var m msg
			err := easyjson.Unmarshal(message, &m)
			if err != nil {
				logger.WithError(err).Errorf("couldn't unmarshal a message from the cloud: %s", string(message))

				continue
			}

			m.Log(logger)
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		select { // check if we should stop before continuing
		case <-ctx.Done():
			return nil
		default:
		}

		if err != nil {
			logger.WithError(err).Warn("error reading a message from the cloud")

			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case msgBuffer <- message:
		}
	}
}
