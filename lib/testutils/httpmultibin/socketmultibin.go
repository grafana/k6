/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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

// Package httpmultibin is indended only for use in tests, do not import in production code!
package httpmultibin

import (
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func getConnectSocketIORequest() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		conn.WriteMessage(websocket.TextMessage, []byte{'0'})
		messageType, r, e := conn.NextReader()
		if e != nil {
			return
		}
		var wc io.WriteCloser
		wc, err = conn.NextWriter(messageType)
		if err != nil {
			return
		}
		if _, err = io.Copy(wc, r); err != nil {
			return
		}
		if err != nil {
			return
		}
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
		// Wait for response control frame
		<-time.After(time.Second)
		err = conn.Close()
		if err != nil {
			return
		}
	})
}

func getSentReceivedSocketIORequest() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		conn.WriteMessage(websocket.TextMessage, []byte{'0'})
		_, data, _ := conn.ReadMessage()
		conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			return
		}
		err = conn.Close()
		if err != nil {
			return
		}
	})
}

func getInvalidCloseSocketHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		conn.WriteMessage(websocket.TextMessage, []byte{'0'})
		messageType, r, e := conn.NextReader()
		if e != nil {
			return
		}
		var wc io.WriteCloser
		wc, err = conn.NextWriter(messageType)
		if err != nil {
			return
		}
		if _, err = io.Copy(wc, r); err != nil {
			return
		}
		if err != nil {
			return
		}
		err = conn.Close()
		if err != nil {
			return
		}
	})
}
