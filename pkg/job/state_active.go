package job

import (
	"io"
	"time"
)

func (a activeHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

func (a activeHandler) status() Status {
	return StatusActive
}

func (a activeHandler) gracefulStop(j *Job, to time.Duration) error {
	j.stopTimer = time.AfterFunc(to, func() {
		_ = j.Stop()
	})

	return j.sendStopSignal(true)
}

func (a activeHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

func (a activeHandler) forceStop(j *Job) error {
	return j.sendStopSignal(false)
}

func (a activeHandler) exited(j *Job) stateHandler {
	close(j.done)
	return endedHandler{}
}
