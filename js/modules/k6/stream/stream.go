package stream

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"sync"
)

type STREAM struct {
	file    *os.File
	reader  *bufio.Reader
	scanner *bufio.Scanner
	csv     *csv.Reader
	loop    bool
	mutex   sync.Mutex
}

func New() *STREAM {
	return &STREAM{}
}

func (stream *STREAM) OpenFile(filename string, loop bool) {
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

func (stream *STREAM) CloseFile() {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

	err := stream.file.Close()
	check(err)
}

func (stream *STREAM) ReadLine() string {
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

func (stream *STREAM) ReadCsvLine() []string {
	stream.mutex.Lock()
	defer stream.mutex.Unlock()

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

func (stream *STREAM) reset() {
	stream.file.Seek(0, 0)
	stream.scanner = bufio.NewScanner(stream.reader)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
