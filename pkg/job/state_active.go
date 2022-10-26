package job

import (
	"io"
	"time"
)

// cleanup purges logs and working dir of the job
func (a activeHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

// status returns current status enum value
func (a activeHandler) status() Status {
	return StatusActive
}

// gracefulStop inits graceful process stop
func (a activeHandler) gracefulStop(j *Job, to time.Duration) error {
	j.stopTimer = time.AfterFunc(to, func() {
		_ = j.Stop()
	})

	return j.sendStopSignal(true)
}

// logs returns a new concurrent reader object to get the job output
func (a activeHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

// forceStop ends the job process immediately, sending SIGKILL
func (a activeHandler) forceStop(j *Job) error {
	return j.sendStopSignal(false)
}

// exited process end event. send internally, when the job's j.cmd.Wait() call returns
func (a activeHandler) exited(j *Job) stateHandler {
	close(j.done)
	return endedHandler{}
}
