package main

import (
	"encoding/json"
	"github.com/loadimpact/speedboat/stats"
	"github.com/loadimpact/speedboat/stats/accumulate"
	"gopkg.in/yaml.v2"
	"io"
)

type Formatter interface {
	Format(data interface{}) ([]byte, error)
}

type JSONFormatter struct{}

func (JSONFormatter) Format(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

type PrettyJSONFormatter struct{}

func (PrettyJSONFormatter) Format(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "    ")
}

type YAMLFormatter struct{}

func (YAMLFormatter) Format(data interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}

type Summarizer struct {
	Accumulator *accumulate.Backend
	Formatter   Formatter
}

func (s *Summarizer) Codify() map[string]interface{} {
	data := make(map[string]interface{})

	for stat, dimensions := range s.Accumulator.Data {
		statData := make(map[string]interface{})

		switch stat.Type {
		case stats.CounterType:
			for dname, dim := range dimensions {
				val := stats.ApplyIntent(dim.Sum(), stat.Intent)
				if len(dimensions) == 1 {
					data[stat.Name] = val
				} else {
					statData[dname] = val
				}
			}
		case stats.GaugeType:
			for dname, dim := range dimensions {
				if dim.Last == 0 {
					continue
				}

				val := stats.ApplyIntent(dim.Last, stat.Intent)
				if len(dimensions) == 1 {
					data[stat.Name] = val
				} else {
					statData[dname] = val
				}
			}
		case stats.HistogramType:
			count := 0
			for dname, dim := range dimensions {
				l := len(dim.Values)
				if l > count {
					count = l
				}

				statData[dname] = map[string]interface{}{
					"min": stats.ApplyIntent(dim.Min(), stat.Intent),
					"max": stats.ApplyIntent(dim.Max(), stat.Intent),
					"avg": stats.ApplyIntent(dim.Avg(), stat.Intent),
					"med": stats.ApplyIntent(dim.Med(), stat.Intent),
				}
			}

			statData["count"] = count
		}

		if len(statData) > 0 {
			data[stat.Name] = statData
		}
	}

	return data
}

func (s *Summarizer) Format() ([]byte, error) {
	return s.Formatter.Format(s.Codify())
}

func (s *Summarizer) Print(w io.Writer) error {
	data, err := s.Format()
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}
	if data[len(data)-1] != '\n' {
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}

	return nil
}
