package streams

import (
	"bufio"
	"encoding/csv"
	"sync"

	"github.com/spf13/afero"
)

type Streams struct {
	fs          afero.Fs
	fileStreams map[string]*FileStream
	mutex       sync.Mutex
}

type Stream struct {
	scanner   *bufio.Scanner
	loop      bool
	mutex     sync.Mutex
	csvHeader []string
}

type FileStream struct {
	Stream
	file afero.File
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

func (fs *FileStream) ReadBytes(bytesToRead int) []byte {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	arr := make([]byte, bytesToRead)
	_, err := fs.file.Read(arr)
	if err != nil {
		panic(err)
	}
	return arr
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
