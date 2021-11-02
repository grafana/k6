/*
 *
 * xk6-browser - a browser automation extension for k6
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

package common

import (
	"os"

	"golang.org/x/net/context"
)

type BrowserProcess struct {
	ctx    context.Context
	cancel context.CancelFunc

	// The process of the browser, if running locally.
	process *os.Process

	// Channels to for managing termination.
	lostConnection             chan struct{}
	processIsGracefullyClosing chan struct{}

	// Browser's WebSocket URL to speak CDP
	wsURL string

	// The directory where user data for the browser is stored.
	userDataDir string
}

func NewBrowserProcess(
	ctx context.Context,
	cancel context.CancelFunc,
	process *os.Process,
	wsURL, userDataDir string,
) *BrowserProcess {
	p := BrowserProcess{
		ctx:                        ctx,
		cancel:                     cancel,
		process:                    process,
		lostConnection:             make(chan struct{}),
		processIsGracefullyClosing: make(chan struct{}),
		wsURL:                      wsURL,
		userDataDir:                userDataDir,
	}
	go func() {
		// If we lose connection to the browser and we're not in-progress with clean
		// browser-initiated termination then cancel the context to clean up.
		<-p.lostConnection
		select {
		case <-p.processIsGracefullyClosing:
		default:
			p.cancel()
		}
	}()
	return &p
}

func (p *BrowserProcess) didLoseConnection() {
	close(p.lostConnection)
}

// GracefulClose triggers a graceful closing of the browser process
func (p *BrowserProcess) GracefulClose() {
	close(p.processIsGracefullyClosing)
}

// Terminate triggers the termination of the browser process
func (p *BrowserProcess) Terminate() {
	p.cancel()
}

// WsURL returns the Websocket URL that the browser is listening on for CDP clients
func (p *BrowserProcess) WsURL() string {
	return p.wsURL
}
