package streams

import (
	"bufio"
	"io"
	"sync"

	"github.com/spf13/afero"
)

type Streams struct {
	readers map[string]StreamReader
	mutex   sync.Mutex
}

type Stream struct {
	mutex sync.Mutex
}

type FileStream struct {
	Stream
	scanner *bufio.Scanner
	fs      afero.Fs
	file    afero.File
}

type StreamReader interface {
	GetStream() *FileStream
	GetReader() io.Reader
	Seek(int64)
	Lock()
	Unlock()
}

func New() *Streams {
	return &Streams{
		readers: make(map[string]StreamReader),
	}
}

func (streams *Streams) OpenFile(filename string, startPos int64, id string) (StreamReader, error) {
	streams.mutex.Lock()
	defer streams.mutex.Unlock()

	// If file is opened with the same args and id return the stored FileStream
	if f, ok := streams.readers[id]; ok {
		return f, nil
	}

	fileStream := &FileStream{}
	fileStream.fs = afero.NewOsFs()
	file, err := fileStream.fs.Open(filename)
	if err != nil {
		return nil, err
	}
	fileStream.file = file
	fileStream.scanner = bufio.NewScanner(fileStream.file)

	if startPos != 0 {
		fileStream.Seek(startPos)
	}

	streams.readers[id] = fileStream

	return fileStream, nil
}

func (s *FileStream) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	err := s.file.Close()
	return err
}

func (s *FileStream) GetStream() *FileStream {
	return s
}

func (s *FileStream) GetReader() io.Reader {
	return s.file
}

func (s *FileStream) Lock() {
	s.Lock()
}

func (s *FileStream) Unlock() {
	s.Unlock()
}

func (s *FileStream) ReadBytes(bytesToRead int) []byte {
	s.Lock()
	defer s.Unlock()

	arr := make([]byte, bytesToRead)
	_, err := s.file.Read(arr)
	if err != nil {
		panic(err)
	}
	return arr
}

// func (fs *Stream) ReadLine() string {
// 	fs.mutex.Lock()
// 	defer fs.mutex.Unlock()

// 	var line string
// 	if fs.scanner.Scan() {
// 		line = fs.scanner.Text()
// 	} else {
// 		err := fs.scanner.Err()
// 		if err != nil {
// 			// An error other than io.EOF occurred
// 			line = err.Error()
// 		} else {
// 			// At end of file
// 			line = fs.scanner.Text()
// 			if fs.loop {
// 				fs.reset(0)
// 			}
// 		}
// 	}
// 	return line
// }

func (s *FileStream) Seek(offset int64) {
	_, err := s.file.Seek(offset, 0)
	check(err)
	s.scanner = bufio.NewScanner(s.file)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
