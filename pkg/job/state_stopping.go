package job

import (
	"fmt"
	"io"
	"time"
)

// status returns current status enum value
func (s stoppingHandler) status() Status {
	return StatusStopping
}

// gracefulStop inits graceful process stop
func (s stoppingHandler) gracefulStop(j *Job, to time.Duration) error {
	return fmt.Errorf("job already stopping")
}

// forceStop ends the job process immediately, sending SIGKILL
func (s stoppingHandler) forceStop(j *Job) error {
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}

	return j.sendStopSignal(false)
}

// cleanup purges logs and working dir of the job
func (s stoppingHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

// logs returns a new concurrent reader object to get the job output
func (s stoppingHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

// exited process end event. send internally, when the job's j.cmd.Wait() call returns
func (s stoppingHandler) exited(j *Job) stateHandler {
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}
	close(j.done)
	return stoppedHandler{}
}
