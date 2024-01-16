// SPDX-FileCopyrightText: 2021 - 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/r3labs/sse/v2"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/errext"
)

type eventEmitter struct {
	*sse.Server
	logger  logrus.FieldLogger
	channel string
	wait    sync.WaitGroup
	id      atomic.Int64
}

var _ eventListener = (*eventEmitter)(nil)

func newEventEmitter(channel string, logger logrus.FieldLogger) *eventEmitter {
	emitter := &eventEmitter{ //nolint:exhaustruct
		channel: channel,
		logger:  logger,
		Server:  sse.New(),
	}

	emitter.CreateStream(channel)

	return emitter
}

func (emitter *eventEmitter) onStart() error {
	return nil
}

func (emitter *eventEmitter) onStop(reason error) error {
	var err errext.HasAbortReason

	if !errors.As(reason, &err) {
		emitter.wait.Wait()
	}

	return nil
}

func (emitter *eventEmitter) onEvent(name string, data interface{}) {
	buff, err := json.Marshal(data)
	if err != nil {
		emitter.logger.Error(err)

		return
	}

	var retry []byte

	if name == stopEvent {
		retry = []byte(strconv.Itoa(maxSafeInteger))
	}

	id := strconv.FormatInt(emitter.id.Add(1), 10)

	emitter.Publish(
		emitter.channel,
		&sse.Event{Event: []byte(name), Data: buff, Retry: retry, ID: []byte(id)},
	) //nolint:exhaustruct
}

func (emitter *eventEmitter) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	values := req.URL.Query()

	values.Add("stream", emitter.channel)
	req.URL.RawQuery = values.Encode()

	res.Header().Set("Access-Control-Allow-Origin", "*")

	emitter.wait.Add(1)
	defer emitter.wait.Done()

	emitter.Server.ServeHTTP(res, req)
}

const maxSafeInteger = 9007199254740991
