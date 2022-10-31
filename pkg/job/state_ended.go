package job

import (
	"fmt"
	"io"
	"time"
)

// status returns current status enum value
func (e endedHandler) status() Status {
	return StatusEnded
}

// gracefulStop inits graceful process stop
func (e endedHandler) gracefulStop(*Job, time.Duration) error {
	return fmt.Errorf("job already ended")
}

// forceStop ends the job process immediately, sending SIGKILL
func (e endedHandler) forceStop(*Job) error {
	return fmt.Errorf("job already ended")
}

// cleanup purges logs and working dir of the job
func (e endedHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

// logs returns a new concurrent reader object to get the job output
func (e endedHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

// exited process end event. send internally, when the job's j.cmd.Wait() call returns
func (e endedHandler) exited(*Job) stateHandler {
	// should never happen
	return endedHandler{}
}
