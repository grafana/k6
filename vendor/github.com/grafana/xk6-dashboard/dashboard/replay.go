// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/browser"
	"github.com/spf13/afero"
)

type replayer struct {
	*eventSource

	reader io.ReadCloser

	options *options

	assets *assets
	proc   *process
}

func replay(input string, opts *options, assets *assets, proc *process) error {
	rep := &replayer{
		options:     opts,
		assets:      assets,
		proc:        proc,
		eventSource: new(eventSource),
	}

	var inputFile afero.File
	var err error

	if inputFile, err = proc.fs.Open(input); err != nil {
		return err
	}

	rep.reader = inputFile

	if strings.HasSuffix(input, gzSuffix) {
		if rep.reader, err = gzip.NewReader(inputFile); err != nil {
			return err
		}

		defer closer(rep.reader, proc.logger)
	}

	defer closer(inputFile, proc.logger)

	return rep.run()
}

func (rep *replayer) run() error {
	rptr := newReporter(rep.options.Export, rep.assets, rep.proc)

	rep.addEventListener(rptr)

	if rep.options.Port >= 0 {
		server := newWebServer(rep.assets.ui, rptr, rep.proc.logger)

		rep.addEventListener(server)

		addr, err := server.listenAndServe(rep.options.addr())
		if err != nil {
			return err
		}

		if rep.options.Port == 0 {
			rep.options.Port = addr.Port
		}

		if rep.options.Open {
			_ = browser.OpenURL(rep.options.url())
		}
	}

	if err := rep.fireStart(); err != nil {
		return err
	}

	rep.fireEvent(configEvent, rep.assets.config)

	decoder := json.NewDecoder(rep.reader)

	for decoder.More() {
		var input replayerEnvelope

		if err := decoder.Decode(&input); err != nil {
			return err
		}

		if input.Name == configEvent {
			continue
		}

		rep.fireEvent(input.Name, input.Data)
	}

	return rep.fireStop(nil)
}

type replayerEnvelope struct {
	Name string      `json:"event"`
	Data interface{} `json:"data"`
}
