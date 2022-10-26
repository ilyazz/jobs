package job

import (
	"fmt"
	"io"
	"time"
)

// status returns current status enum value
func (z zombieHandler) status() Status {
	// should never happen
	return StatusRemoved
}

// forceStop ends the job process immediately, sending SIGKILL
func (z zombieHandler) forceStop(*Job) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

// gracefulStop inits graceful process stop
func (z zombieHandler) gracefulStop(*Job, time.Duration) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

// cleanup purges logs and working dir of the job
func (z zombieHandler) cleanup(*Job) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

// logs returns a new concurrent reader object to get the job output
func (z zombieHandler) logs(*Job) (io.ReadCloser, error) {
	// should never happen
	return nil, fmt.Errorf("job is removed")
}

// exited process end event. send internally, when the job's j.cmd.Wait() call returns
func (z zombieHandler) exited(*Job) stateHandler {
	// should never happen
	return zombieHandler{}
}
