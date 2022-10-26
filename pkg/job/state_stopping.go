package job

import (
	"fmt"
	"io"
	"time"
)

func (s stoppingHandler) status() Status {
	return StatusStopping
}

func (s stoppingHandler) gracefulStop(j *Job, to time.Duration) error {
	return fmt.Errorf("job already stopping")
}

func (s stoppingHandler) forceStop(j *Job) error {
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}

	return j.sendStopSignal(false)
}

func (s stoppingHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

func (s stoppingHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

func (s stoppingHandler) exited(j *Job) stateHandler {
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}
	close(j.done)
	return stoppedHandler{}
}
