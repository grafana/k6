package streams

import "io"

type struct CSVStream {
	csv       *csv.Reader,
}

func (fs *FileStream) GetHeaders() []string {
	return fs.csvHeader
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

func (fs *FileStream) ReadCSVLine() []string {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	return fs.readCSVLine()
}
