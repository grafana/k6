package js

import (
	"os"
	"sync"
)

type keyloggerWriter struct {
	f *os.File
	m *sync.Mutex
}

func (k *keyloggerWriter) Write(b []byte) (int, error) {
	k.m.Lock()
	defer k.m.Unlock()
	return k.f.Write(b)
}
