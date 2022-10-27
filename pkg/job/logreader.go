package job

import (
	"io"
	"sync"
	"sync/atomic"
)

// outputReader provides access to job combined output, implementing io.ReadCloser.
type outputReader struct {
	// source
	f io.ReadCloser
	// to sync with cleanup. until lock is busy, job output cannot be deleted
	lock    *sync.WaitGroup
	counter *int32
}

// Read reads at most len(b) bytes into b, returns the number of read bytes.
func (r *outputReader) Read(b []byte) (int, error) {
	return r.f.Read(b)
}

// Close closes the source file.
func (r *outputReader) Close() error {
	defer r.lock.Done()
	atomic.AddInt32(r.counter, -1)

	return r.f.Close()
}
