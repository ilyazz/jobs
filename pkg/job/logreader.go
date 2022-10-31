package job

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// ErrEOFJobDone indicates end of output is reached and the job process exited
var ErrEOFJobDone = fmt.Errorf("end of output : %w", io.EOF)

// outputReader provides access to job combined output, implementing io.ReadCloser.
type outputReader struct {
	// source
	f io.ReadCloser
	// to sync with cleanup. until lock is busy, job output cannot be deleted
	lock    *sync.WaitGroup
	counter *int32
	done    chan struct{}
}

// Read reads at most len(b) bytes into b, returns the number of read bytes.
func (r *outputReader) Read(b []byte) (int, error) {
	n, err := r.f.Read(b)
	if err != io.EOF {
		return n, err
	}

	select {
	case <-r.done:
		return n, ErrEOFJobDone
	default:
	}

	return n, err
}

// Close closes the source file.
func (r *outputReader) Close() error {
	defer r.lock.Done()
	atomic.AddInt32(r.counter, -1)

	return r.f.Close()
}
