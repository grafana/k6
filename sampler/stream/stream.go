package stream

import (
	"encoding/csv"
	"encoding/json"
	"github.com/loadimpact/speedboat/sampler"
	"io"
	"strconv"
	"sync"
)

type JSONOutput struct {
	Output io.WriteCloser

	encoder *json.Encoder
	mutex   sync.Mutex
}

func (o *JSONOutput) Write(m *sampler.Metric, e *sampler.Entry) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.encoder == nil {
		o.encoder = json.NewEncoder(o.Output)
	}
	if err := o.encoder.Encode(e); err != nil {
		return err
	}
	return nil
}

func (o *JSONOutput) Commit() error {
	return nil
}

func (o *JSONOutput) Close() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.Output.Close()
}

type CSVOutput struct {
	Output io.WriteCloser

	writer *csv.Writer
	mutex  sync.Mutex
}

func (o *CSVOutput) Write(m *sampler.Metric, e *sampler.Entry) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.writer == nil {
		o.writer = csv.NewWriter(o.Output)
	}

	record := []string{m.Name, strconv.FormatInt(e.Value, 10)}
	if err := o.writer.Write(record); err != nil {
		return err
	}
	return nil
}

func (o *CSVOutput) Commit() error {
	o.writer.Flush()
	return nil
}

func (o *CSVOutput) Close() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.Output.Close()
}
