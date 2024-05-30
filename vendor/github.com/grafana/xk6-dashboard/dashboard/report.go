// SPDX-FileCopyrightText: 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

package dashboard

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"sync"
)

type reporter struct {
	assets *assets
	proc   *process

	data   *reportData
	output string
	mu     sync.RWMutex

	snapshotCount int
}

var (
	_ eventListener = (*reporter)(nil)
	_ http.Handler  = (*reporter)(nil)
)

func newReporter(output string, assets *assets, proc *process) *reporter {
	rep := &reporter{ //nolint:exhaustruct
		data:   newReportData(assets.config),
		assets: assets,
		proc:   proc,
		output: output,
	}

	return rep
}

func (rep *reporter) onStart() error {
	return nil
}

func (rep *reporter) onStop(_ error) error {
	if len(rep.output) == 0 {
		return nil
	}

	if rep.snapshotCount < 2 {
		rep.proc.logger.Warn(
			"The test run was short, report generation was skipped (not enough data)",
		)
		return nil
	}

	file, err := rep.proc.fs.Create(rep.output)
	if err != nil {
		return err
	}

	compress := filepath.Ext(rep.output) == ".gz"

	var out io.WriteCloser = file

	if compress {
		out = gzip.NewWriter(file)
	}

	if err := rep.exportHTML(out); err != nil {
		return err
	}

	if compress {
		if err := out.Close(); err != nil {
			return err
		}
	}

	return file.Close()
}

func (rep *reporter) onEvent(name string, data interface{}) {
	rep.mu.Lock()
	defer rep.mu.Unlock()

	if name == configEvent {
		return
	}

	envelope := &recorderEnvelope{Name: name, Data: data}

	if name == cumulativeEvent {
		rep.data.cumulative = envelope

		return
	}

	if name == thresholdEvent {
		rep.data.threshold = envelope

		return
	}

	if err := rep.data.encoder.Encode(envelope); err != nil {
		if eerr := rep.data.encoder.Encode(nil); eerr != nil {
			rep.proc.logger.Error(err)
		}

		rep.proc.logger.Error(err)

		return
	}

	if name == snapshotEvent {
		rep.snapshotCount++
	}
}

func (rep *reporter) ServeHTTP(res http.ResponseWriter, _ *http.Request) {
	res.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := rep.exportHTML(res); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func (rep *reporter) exportJSON(out io.Writer) error {
	rep.mu.RLock()
	defer rep.mu.RUnlock()

	return rep.data.exportJSON(out)
}

func (rep *reporter) exportBase64(out io.Writer) error {
	outB64 := base64.NewEncoder(base64.StdEncoding, out)
	outGZ := gzip.NewWriter(outB64)

	if err := rep.exportJSON(outGZ); err != nil {
		return err
	}

	if err := outGZ.Close(); err != nil {
		return err
	}

	return outB64.Close()
}

func (rep *reporter) exportHTML(out io.Writer) error {
	file, err := rep.assets.report.Open("index.html")
	if err != nil {
		return err
	}

	html, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	html, err = rep.inject(out, html, []byte(dataTag), rep.exportBase64)
	if err != nil {
		return err
	}

	if _, err := out.Write(html); err != nil {
		return err
	}

	return nil
}

func (rep *reporter) inject(
	out io.Writer,
	html []byte,
	tag []byte,
	dataFunc func(io.Writer) error,
) ([]byte, error) {
	idx := bytes.Index(html, tag)

	if idx < 0 {
		panic("invalid brief HTML, no tag: " + string(tag))
	}

	idx += len(tag)

	if _, err := out.Write(html[:idx]); err != nil {
		return nil, err
	}

	if err := dataFunc(out); err != nil {
		return nil, err
	}

	return html[idx:], nil
}

type reportData struct {
	config     *recorderEnvelope
	buff       bytes.Buffer
	encoder    *json.Encoder
	cumulative *recorderEnvelope
	threshold  *recorderEnvelope
}

func newReportData(config json.RawMessage) *reportData {
	data := new(reportData)

	if config != nil {
		data.config = &recorderEnvelope{Name: configEvent, Data: config}
	}

	data.encoder = json.NewEncoder(&data.buff)

	return data
}

func (data *reportData) exportJSON(out io.Writer) error {
	encoder := json.NewEncoder(out)

	if data.config != nil {
		if err := encoder.Encode(data.config); err != nil {
			return err
		}
	}

	if data.buff.Len() != 0 {
		if _, err := out.Write(data.buff.Bytes()); err != nil {
			return err
		}
	}

	if data.cumulative != nil {
		if err := encoder.Encode(data.cumulative); err != nil {
			return err
		}
	}

	if data.threshold != nil {
		return encoder.Encode(data.threshold)
	}

	return nil
}

const dataTag = `<script id="data" type="application/json; charset=utf-8; gzip; base64">`
