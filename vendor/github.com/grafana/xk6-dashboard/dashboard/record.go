// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"strings"
	"sync"
)

type recorder struct {
	output  string
	proc    *process
	mu      sync.RWMutex
	encoder *json.Encoder
	writer  io.WriteCloser
}

var _ eventListener = (*recorder)(nil)

func newRecorder(output string, proc *process) *recorder {
	rec := &recorder{
		output: output,
		proc:   proc,
	}

	return rec
}

func (rec *recorder) onStart() error {
	file, err := rec.proc.fs.Create(rec.output)
	if err != nil {
		return err
	}

	rec.writer = file

	if strings.HasSuffix(rec.output, ".gz") {
		rec.writer = gzip.NewWriter(file)
	}

	rec.encoder = json.NewEncoder(rec.writer)

	return nil
}

func (rec *recorder) onStop(_ error) error {
	return rec.writer.Close()
}

func (rec *recorder) onEvent(name string, data interface{}) {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if name == configEvent {
		return
	}

	event := &recorderEnvelope{Name: name, Data: data}

	if err := rec.encoder.Encode(event); err != nil {
		rec.proc.logger.Warn(err)
	}
}

type recorderEnvelope struct {
	Name string      `json:"event"`
	Data interface{} `json:"data"`
}
