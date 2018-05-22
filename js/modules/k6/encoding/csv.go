package encoding

import (
	"encoding/csv"
	"io"

	"github.com/loadimpact/k6/js/streams"
)

type CSV struct {
}

type CSVStream struct {
	streams.StreamIO
	csvReader *csv.Reader
	csvHeader []string
	loop      bool
}

func (_ *CSV) New(streamio streams.StreamIO, header bool, loop bool) *CSVStream {
	s := &CSVStream{
		streamio,
		csv.NewReader(streamio.GetReader()),
		nil,
		loop,
	}
	if header {
		s.csvHeader = s.readCSVLine()
	}
	return s
}

func (s *CSVStream) GetHeaders() []string {
	return s.csvHeader
}

func (s *CSVStream) ReadCSVLine() []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return fs.readCSVLine()
}

func (csv *CSVStream) readCSVLine() []string {
	out, err := fs.csv.Read()
	if err == io.EOF {
		if fs.loop {
			fs.reset(0)
		}
	} else if err != nil {
		panic(err)
	}
	return out
}

func (fs *CSVStream) readCSVHeader() []string {
	line := fs.readCSVLine()
	fs.csvHeader = line
	return line
}
