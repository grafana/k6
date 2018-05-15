package streams

import (
	"bufio"
	"encoding/csv"
	"io"
	"sync"

	"github.com/spf13/afero"
)

type Streams struct {
	fs    afero.Fs
	files map[string]*FileStream
	mutex sync.Mutex
}

type FileStream struct {
	file      afero.File
	scanner   *bufio.Scanner
	csv       *csv.Reader
	loop      bool
	mutex     sync.Mutex
	csvHeader []string
}

func New(fs afero.Fs) *Streams {
	return &Streams{
		files: make(map[string]*FileStream),
		fs:    fs,
	}
}

func (streams *Streams) OpenFile(filename string, loop bool, header bool, startPos int64, id string) (*FileStream, error) {
	streams.mutex.Lock()
	defer streams.mutex.Unlock()

	// If file is opened with the same args and id return the stored FileStream
	if f, ok := streams.files[id]; ok {
		return f, nil
	}

	f, err := streams.fs.Open(filename)
	if err != nil {
		return nil, err
	}
	fileStream := &FileStream{}
	fileStream.file = f
	fileStream.scanner = bufio.NewScanner(f)
	fileStream.csv = csv.NewReader(f)
	fileStream.loop = loop

	if header {
		fileStream.readCSVHeader()
	}

	if startPos != 0 {
		fileStream.reset(startPos)
	}

	streams.files[id] = fileStream

	return fileStream, nil
}

func (fs *FileStream) Close() error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	err := fs.file.Close()
	return err
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
			// An error other than io.EOF occurred
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
	_, err := fs.file.Seek(offset, 0)
	check(err)
	fs.scanner = bufio.NewScanner(fs.file)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
