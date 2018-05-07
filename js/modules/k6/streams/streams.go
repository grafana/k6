package streams

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sync"
)

type Streams struct {
	files map[string]*FileStream
	mutex sync.Mutex
}

type FileStream struct {
	ctx       context.Context
	file      *os.File
	reader    *bufio.Reader
	scanner   *bufio.Scanner
	csv       *csv.Reader
	loop      bool
	mutex     sync.Mutex
	csvHeader []string
}

func New() *Streams {
	return &Streams{files: make(map[string]*FileStream)}
}

func (streams *Streams) File(id string) *FileStream {
	return streams.files[id]
}

func (streams *Streams) OpenFile(filename string, loop bool, headerLine bool, startPos int64) string {
	streams.mutex.Lock()
	defer streams.mutex.Unlock()

	f, err := os.Open(filename)
	check(err)
	fileStream := &FileStream{}
	fileStream.file = f
	fileStream.reader = bufio.NewReader(fileStream.file)
	fileStream.scanner = bufio.NewScanner(fileStream.reader)
	fileStream.csv = csv.NewReader(fileStream.reader)
	fileStream.loop = loop

	if headerLine {
		fileStream.readCSVHeader()
	}

	if startPos != 0 {
		fileStream.reset(startPos)
	}

	var key = fmt.Sprintf(`%s/%t/%t/%d`, filename, loop, headerLine, startPos)

	streams.files[key] = fileStream

	return key
}

func (fs *FileStream) Close() {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	err := fs.file.Close()
	check(err)
}

func (fs *FileStream) ReadLine() string {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	var line string
	if fs.scanner.Scan() {
		line = fs.scanner.Text()
	} else {
		err := fs.scanner.Err()
		if err != nil {
			// An error other than io.EOF occured
			line = err.Error()
		} else {
			// At end of file
			line = fs.scanner.Text()
			if fs.loop {
				fs.reset(0)
			}
		}
	}
	return line
}

func (fs *FileStream) GetHeaders() []string {
	return fs.csvHeader
}

func (fs *FileStream) ReadCSVLine() []string {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	return fs.readCSVLine()
}

func (fs *FileStream) readCSVLine() []string {
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

func (fs *FileStream) readCSVHeader() []string {
	line := fs.readCSVLine()
	fs.csvHeader = line
	return line
}

func (fs *FileStream) reset(offset int64) {
	fs.file.Seek(offset, 0)
	fs.scanner = bufio.NewScanner(fs.reader)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
