/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sse

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/cenkalti/backoff.v1"
)

var (
	headerID    = []byte("id:")
	headerData  = []byte("data:")
	headerEvent = []byte("event:")
	headerRetry = []byte("retry:")
)

func ClientMaxBufferSize(s int) func(c *Client) {
	return func(c *Client) {
		c.maxBufferSize = s
	}
}

// ConnCallback defines a function to be called on a particular connection event
type ConnCallback func(c *Client)

// ResponseValidator validates a response
type ResponseValidator func(c *Client, resp *http.Response) error

// Client handles an incoming server stream
type Client struct {
	Retry             time.Time
	ReconnectStrategy backoff.BackOff
	disconnectcb      ConnCallback
	connectedcb       ConnCallback
	subscribed        map[chan *Event]chan struct{}
	Headers           map[string]string
	ReconnectNotify   backoff.Notify
	ResponseValidator ResponseValidator
	Connection        *http.Client
	URL               string
	LastEventID       atomic.Value // []byte
	maxBufferSize     int
	mu                sync.Mutex
	EncodingBase64    bool
	Connected         bool
}

// NewClient creates a new client
func NewClient(url string, opts ...func(c *Client)) *Client {
	c := &Client{
		URL:           url,
		Connection:    &http.Client{},
		Headers:       make(map[string]string),
		subscribed:    make(map[chan *Event]chan struct{}),
		maxBufferSize: 1 << 16,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Subscribe to a data stream
func (c *Client) Subscribe(stream string, handler func(msg *Event)) error {
	return c.SubscribeWithContext(context.Background(), stream, handler)
}

// SubscribeWithContext to a data stream with context
func (c *Client) SubscribeWithContext(ctx context.Context, stream string, handler func(msg *Event)) error {
	operation := func() error {
		resp, err := c.request(ctx, stream)
		if err != nil {
			return err
		}
		if validator := c.ResponseValidator; validator != nil {
			err = validator(c, resp)
			if err != nil {
				return err
			}
		} else if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode))
		}
		defer resp.Body.Close()

		reader := NewEventStreamReader(resp.Body, c.maxBufferSize)
		eventChan, errorChan := c.startReadLoop(reader)

		for {
			select {
			case err = <-errorChan:
				return err
			case msg := <-eventChan:
				handler(msg)
			}
		}
	}

	// Apply user specified reconnection strategy or default to standard NewExponentialBackOff() reconnection method
	var err error
	if c.ReconnectStrategy != nil {
		err = backoff.RetryNotify(operation, c.ReconnectStrategy, c.ReconnectNotify)
	} else {
		err = backoff.RetryNotify(operation, backoff.NewExponentialBackOff(), c.ReconnectNotify)
	}
	return err
}

// SubscribeChan sends all events to the provided channel
func (c *Client) SubscribeChan(stream string, ch chan *Event) error {
	return c.SubscribeChanWithContext(context.Background(), stream, ch)
}

// SubscribeChanWithContext sends all events to the provided channel with context
func (c *Client) SubscribeChanWithContext(ctx context.Context, stream string, ch chan *Event) error {
	var connected bool
	errch := make(chan error)
	c.mu.Lock()
	c.subscribed[ch] = make(chan struct{})
	c.mu.Unlock()

	operation := func() error {
		resp, err := c.request(ctx, stream)
		if err != nil {
			return err
		}
		if validator := c.ResponseValidator; validator != nil {
			err = validator(c, resp)
			if err != nil {
				return err
			}
		} else if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("could not connect to stream: %s", http.StatusText(resp.StatusCode))
		}
		defer resp.Body.Close()

		if !connected {
			// Notify connect
			errch <- nil
			connected = true
		}

		reader := NewEventStreamReader(resp.Body, c.maxBufferSize)
		eventChan, errorChan := c.startReadLoop(reader)

		for {
			var msg *Event
			// Wait for message to arrive or exit
			select {
			case <-c.subscribed[ch]:
				return nil
			case err = <-errorChan:
				return err
			case msg = <-eventChan:
			}

			// Wait for message to be sent or exit
			if msg != nil {
				select {
				case <-c.subscribed[ch]:
					return nil
				case ch <- msg:
					// message sent
				}
			}
		}
	}

	go func() {
		defer c.cleanup(ch)
		// Apply user specified reconnection strategy or default to standard NewExponentialBackOff() reconnection method
		var err error
		if c.ReconnectStrategy != nil {
			err = backoff.RetryNotify(operation, c.ReconnectStrategy, c.ReconnectNotify)
		} else {
			err = backoff.RetryNotify(operation, backoff.NewExponentialBackOff(), c.ReconnectNotify)
		}

		// channel closed once connected
		if err != nil && !connected {
			errch <- err
		}
	}()
	err := <-errch
	close(errch)
	return err
}

func (c *Client) startReadLoop(reader *EventStreamReader) (chan *Event, chan error) {
	outCh := make(chan *Event)
	erChan := make(chan error)
	go c.readLoop(reader, outCh, erChan)
	return outCh, erChan
}

func (c *Client) readLoop(reader *EventStreamReader, outCh chan *Event, erChan chan error) {
	for {
		// Read each new line and process the type of event
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				erChan <- nil
				return
			}
			// run user specified disconnect function
			if c.disconnectcb != nil {
				c.Connected = false
				c.disconnectcb(c)
			}
			erChan <- err
			return
		}

		if !c.Connected && c.connectedcb != nil {
			c.Connected = true
			c.connectedcb(c)
		}

		// If we get an error, ignore it.
		var msg *Event
		if msg, err = c.processEvent(event); err == nil {
			if len(msg.ID) > 0 {
				c.LastEventID.Store(msg.ID)
			} else {
				msg.ID, _ = c.LastEventID.Load().([]byte)
			}

			// Send downstream if the event has something useful
			if msg.hasContent() {
				outCh <- msg
			}
		}
	}
}

// SubscribeRaw to an sse endpoint
func (c *Client) SubscribeRaw(handler func(msg *Event)) error {
	return c.Subscribe("", handler)
}

// SubscribeRawWithContext to an sse endpoint with context
func (c *Client) SubscribeRawWithContext(ctx context.Context, handler func(msg *Event)) error {
	return c.SubscribeWithContext(ctx, "", handler)
}

// SubscribeChanRaw sends all events to the provided channel
func (c *Client) SubscribeChanRaw(ch chan *Event) error {
	return c.SubscribeChan("", ch)
}

// SubscribeChanRawWithContext sends all events to the provided channel with context
func (c *Client) SubscribeChanRawWithContext(ctx context.Context, ch chan *Event) error {
	return c.SubscribeChanWithContext(ctx, "", ch)
}

// Unsubscribe unsubscribes a channel
func (c *Client) Unsubscribe(ch chan *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscribed[ch] != nil {
		c.subscribed[ch] <- struct{}{}
	}
}

// OnDisconnect specifies the function to run when the connection disconnects
func (c *Client) OnDisconnect(fn ConnCallback) {
	c.disconnectcb = fn
}

// OnConnect specifies the function to run when the connection is successful
func (c *Client) OnConnect(fn ConnCallback) {
	c.connectedcb = fn
}

func (c *Client) request(ctx context.Context, stream string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.URL, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)

	// Setup request, specify stream to connect to
	if stream != "" {
		query := req.URL.Query()
		query.Add("stream", stream)
		req.URL.RawQuery = query.Encode()
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	lastID, exists := c.LastEventID.Load().([]byte)
	if exists && lastID != nil {
		req.Header.Set("Last-Event-ID", string(lastID))
	}

	// Add user specified headers
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	return c.Connection.Do(req)
}

func (c *Client) processEvent(msg []byte) (event *Event, err error) {
	var e Event

	if len(msg) < 1 {
		return nil, errors.New("event message was empty")
	}

	// Normalize the crlf to lf to make it easier to split the lines.
	// Split the line by "\n" or "\r", per the spec.
	for _, line := range bytes.FieldsFunc(msg, func(r rune) bool { return r == '\n' || r == '\r' }) {
		switch {
		case bytes.HasPrefix(line, headerID):
			e.ID = append([]byte(nil), trimHeader(len(headerID), line)...)
		case bytes.HasPrefix(line, headerData):
			// The spec allows for multiple data fields per event, concatenated them with "\n".
			e.Data = append(e.Data[:], append(trimHeader(len(headerData), line), byte('\n'))...)
		// The spec says that a line that simply contains the string "data" should be treated as a data field with an empty body.
		case bytes.Equal(line, bytes.TrimSuffix(headerData, []byte(":"))):
			e.Data = append(e.Data, byte('\n'))
		case bytes.HasPrefix(line, headerEvent):
			e.Event = append([]byte(nil), trimHeader(len(headerEvent), line)...)
		case bytes.HasPrefix(line, headerRetry):
			e.Retry = append([]byte(nil), trimHeader(len(headerRetry), line)...)
		default:
			// Ignore any garbage that doesn't match what we're looking for.
		}
	}

	// Trim the last "\n" per the spec.
	e.Data = bytes.TrimSuffix(e.Data, []byte("\n"))

	if c.EncodingBase64 {
		buf := make([]byte, base64.StdEncoding.DecodedLen(len(e.Data)))

		n, err := base64.StdEncoding.Decode(buf, e.Data)
		if err != nil {
			err = fmt.Errorf("failed to decode event message: %s", err)
		}
		e.Data = buf[:n]
	}
	return &e, err
}

func (c *Client) cleanup(ch chan *Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.subscribed[ch] != nil {
		close(c.subscribed[ch])
		delete(c.subscribed, ch)
	}
}

func trimHeader(size int, data []byte) []byte {
	if data == nil || len(data) < size {
		return data
	}

	data = data[size:]
	// Remove optional leading whitespace
	if len(data) > 0 && data[0] == 32 {
		data = data[1:]
	}
	// Remove trailing new line
	if len(data) > 0 && data[len(data)-1] == 10 {
		data = data[:len(data)-1]
	}
	return data
}
