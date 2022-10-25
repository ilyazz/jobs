package job

import (
	"github.com/spf13/afero"
	"sync"
	"sync/atomic"
)

// outputReader provides access to job combined output, implementing io.ReadCloser
type outputReader struct {
	// source file
	f afero.File
	// when reached the eof, should we wait for more data
	follow bool
	// is the job process ended
	done *atomic.Bool
	// to sync with cleanup. until lock is busy, job output cannot be deleted
	lock *sync.WaitGroup
}

// Read reads at most len(b) bytes into b, returns the number of read bytes,
func (r *outputReader) Read(b []byte) (int, error) {
	return r.f.Read(b)
	//n, err := r.f.Read(b)
	//if err == io.EOF {
	//	if !r.done.Load() && r.follow {
	//		return n, nil
	//	}
	//}
	//
	//return n, err
}

// Close closes the source file
func (r *outputReader) Close() error {
	defer r.lock.Done()
	return r.f.Close()
}
