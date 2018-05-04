package stream

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"sync"
)

type Stream FileStream

type FileStream struct {
	file      *os.File
	reader    *bufio.Reader
	scanner   *bufio.Scanner
	csv       *csv.Reader
	loop      bool
	mutex     sync.Mutex
	csvHeader []string
}

func New() *FileStream {
	return &FileStream{}
}

func (stream *FileStream) OpenFile(filename string, loop bool) {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	f, err := os.Open(filename)
	check(err)
	stream.file = f
	stream.reader = bufio.NewReader(stream.file)
	stream.scanner = bufio.NewScanner(stream.reader)
	stream.csv = csv.NewReader(stream.reader)
	stream.loop = loop
}

func (stream *FileStream) CloseFile() {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	err := stream.file.Close()
	check(err)
}

func (stream *FileStream) ReadLine() string {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	var line string
	if stream.scanner.Scan() {
		line = stream.scanner.Text()
	} else {
		err := stream.scanner.Err()
		if err != nil {
			// An error other than io.EOF occured
			line = err.Error()
		} else {
			// At end of file
			line = stream.scanner.Text()
			if stream.loop {
				stream.reset()
			}
		}
	}
	return line
}

func (stream *FileStream) ReadCSVHeader() []string {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	line := stream.readCSVLine()
	stream.csvHeader = line
	return line
}

func (stream *FileStream) ReadCSVLine() []string {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	return stream.readCSVLine()
}

func (stream *FileStream) readCSVLine() []string {
	out, err := stream.csv.Read()
	if err == io.EOF {
		if stream.loop {
			stream.reset()
		}
	} else if err != nil {
		panic(err)
	}
	return out
}

func (stream *FileStream) reset() {
	stream.file.Seek(0, 0)
	stream.scanner = bufio.NewScanner(stream.reader)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
