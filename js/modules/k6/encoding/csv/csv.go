package csv

import (
	"encoding/csv"
	"io"
	"sync"

	"github.com/loadimpact/k6/js/streams"
)

type CSV struct {
	streams map[string]*CSVStream
	mutes   sync.Mutex
}

type CSVStream struct {
	streams.Stream
	csvReader *csv.Reader
	csvHeader []string
	loop      bool
}

func New() *CSV {
	return &CSV{streams: make(map[string]*CSVStream)}
}

func (c *CSV) NewStream(stream streams.Stream /*interface{}*/, header bool, loop bool) *CSVStream {
	if s, ok := c.streams[stream.GetId()]; ok {
		return s
	}

	s := &CSVStream{
		stream,
		csv.NewReader(stream.GetReader()),
		nil,
		loop,
	}

	if header {
		s.csvHeader = s.readCSVLine()
	}

	c.streams[stream.GetId()] = s

	return s
}

func (s *CSVStream) GetHeaders() []string {
	return s.csvHeader
}

func (s *CSVStream) ReadCSVLine() []string {
	s.Lock()
	defer s.Unlock()

	return s.readCSVLine()
}

func (s *CSVStream) readCSVLine() []string {
	out, err := s.csvReader.Read()
	if err == io.EOF {
		if s.loop {
			s.Seek(0)
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
