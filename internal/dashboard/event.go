// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

type eventListener interface {
	onEvent(event string, data any)
	onStart() error
	onStop(reason error) error
}

type eventSource struct {
	listeners []eventListener
}

func (src *eventSource) addEventListener(listener eventListener) {
	src.listeners = append(src.listeners, listener)
}

func (src *eventSource) fireEvent(event string, data any) {
	for _, e := range src.listeners {
		e.onEvent(event, data)
	}
}

func (src *eventSource) fireStart() error {
	for _, e := range src.listeners {
		if err := e.onStart(); err != nil {
			return err
		}
	}

	return nil
}

func (src *eventSource) fireStop(reason error) error {
	for _, e := range src.listeners {
		if err := e.onStop(reason); err != nil {
			return err
		}
	}

	return nil
}
