/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sse

import (
	"encoding/base64"
	"sync"
	"time"
)

// DefaultBufferSize size of the queue that holds the streams messages.
const DefaultBufferSize = 1024

// Server Is our main struct
type Server struct {
	// Extra headers adding to the HTTP response to each client
	Headers map[string]string
	// Sets a ttl that prevents old events from being transmitted
	EventTTL time.Duration
	// Specifies the size of the message buffer for each stream
	BufferSize int
	// Encodes all data as base64
	EncodeBase64 bool
	// Splits an events data into multiple data: entries
	SplitData bool
	// Enables creation of a stream when a client connects
	AutoStream bool
	// Enables automatic replay for each new subscriber that connects
	AutoReplay bool

	// Specifies the function to run when client subscribe or un-subscribe
	OnSubscribe   func(streamID string, sub *Subscriber)
	OnUnsubscribe func(streamID string, sub *Subscriber)

	streams   map[string]*Stream
	muStreams sync.RWMutex
}

// New will create a server and setup defaults
func New() *Server {
	return &Server{
		BufferSize: DefaultBufferSize,
		AutoStream: false,
		AutoReplay: true,
		streams:    make(map[string]*Stream),
		Headers:    map[string]string{},
	}
}

// NewWithCallback will create a server and setup defaults with callback function
func NewWithCallback(onSubscribe, onUnsubscribe func(streamID string, sub *Subscriber)) *Server {
	return &Server{
		BufferSize:    DefaultBufferSize,
		AutoStream:    false,
		AutoReplay:    true,
		streams:       make(map[string]*Stream),
		Headers:       map[string]string{},
		OnSubscribe:   onSubscribe,
		OnUnsubscribe: onUnsubscribe,
	}
}

// Close shuts down the server, closes all of the streams and connections
func (s *Server) Close() {
	s.muStreams.Lock()
	defer s.muStreams.Unlock()

	for id := range s.streams {
		s.streams[id].close()
		delete(s.streams, id)
	}
}

// CreateStream will create a new stream and register it
func (s *Server) CreateStream(id string) *Stream {
	s.muStreams.Lock()
	defer s.muStreams.Unlock()

	if s.streams[id] != nil {
		return s.streams[id]
	}

	str := newStream(id, s.BufferSize, s.AutoReplay, s.AutoStream, s.OnSubscribe, s.OnUnsubscribe)
	str.run()

	s.streams[id] = str

	return str
}

// RemoveStream will remove a stream
func (s *Server) RemoveStream(id string) {
	s.muStreams.Lock()
	defer s.muStreams.Unlock()

	if s.streams[id] != nil {
		s.streams[id].close()
		delete(s.streams, id)
	}
}

// StreamExists checks whether a stream by a given id exists
func (s *Server) StreamExists(id string) bool {
	return s.getStream(id) != nil
}

// Publish sends a mesage to every client in a streamID.
// If the stream's buffer is full, it blocks until the message is sent out to
// all subscribers (but not necessarily arrived the clients), or when the
// stream is closed.
func (s *Server) Publish(id string, event *Event) {
	stream := s.getStream(id)
	if stream == nil {
		return
	}

	select {
	case <-stream.quit:
	case stream.event <- s.process(event):
	}
}

// TryPublish is the same as Publish except that when the operation would cause
// the call to be blocked, it simply drops the message and returns false.
// Together with a small BufferSize, it can be useful when publishing the
// latest message ASAP is more important than reliable delivery.
func (s *Server) TryPublish(id string, event *Event) bool {
	stream := s.getStream(id)
	if stream == nil {
		return false
	}

	select {
	case stream.event <- s.process(event):
		return true
	default:
		return false
	}
}

func (s *Server) getStream(id string) *Stream {
	s.muStreams.RLock()
	defer s.muStreams.RUnlock()
	return s.streams[id]
}

func (s *Server) process(event *Event) *Event {
	if s.EncodeBase64 {
		output := make([]byte, base64.StdEncoding.EncodedLen(len(event.Data)))
		base64.StdEncoding.Encode(output, event.Data)
		event.Data = output
	}
	return event
}
