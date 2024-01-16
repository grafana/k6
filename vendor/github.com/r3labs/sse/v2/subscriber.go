/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sse

import "net/url"

// Subscriber ...
type Subscriber struct {
	quit       chan *Subscriber
	connection chan *Event
	removed    chan struct{}
	eventid    int
	URL        *url.URL
}

// Close will let the stream know that the clients connection has terminated
func (s *Subscriber) close() {
	s.quit <- s
	if s.removed != nil {
		<-s.removed
	}
}
