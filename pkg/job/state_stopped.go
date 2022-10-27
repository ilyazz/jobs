package job

import (
	"fmt"
	"io"
	"time"
)

// status returns current status enum value
func (s stoppedHandler) status() Status {
	return StatusStopped
}

// gracefulStop inits graceful process stop
func (s stoppedHandler) gracefulStop(*Job, time.Duration) error {
	return fmt.Errorf("job is already stopped")
}

// forceStop ends the job process immediately, sending SIGKILL
func (s stoppedHandler) forceStop(*Job) error {
	return fmt.Errorf("job is already stopped")
}

// cleanup purges logs and working dir of the job
func (s stoppedHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

// logs returns a new concurrent reader object to get the job output
func (s stoppedHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

// exited process end event. send internally, when the job's j.cmd.Wait() call returns
func (s stoppedHandler) exited(j *Job) stateHandler {
	close(j.done)
	return stoppedHandler{}
}
