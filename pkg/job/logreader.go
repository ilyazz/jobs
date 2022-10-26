package job

import (
	"github.com/spf13/afero"
	"sync"
)

// outputReader provides access to job combined output, implementing io.ReadCloser.
type outputReader struct {
	// source file
	f afero.File
	// to sync with cleanup. until lock is busy, job output cannot be deleted
	lock *sync.WaitGroup
}

// Read reads at most len(b) bytes into b, returns the number of read bytes.
func (r *outputReader) Read(b []byte) (int, error) {
	return r.f.Read(b)
}

// Close closes the source file.
func (r *outputReader) Close() error {
	defer r.lock.Done()
	return r.f.Close()
}
