package job

import (
	"fmt"
	"io"
	"time"
)

func (s stoppedHandler) status() Status {
	return StatusStopped
}

func (s stoppedHandler) gracefulStop(*Job, time.Duration) error {
	return fmt.Errorf("job is already stopped")
}

func (s stoppedHandler) forceStop(*Job) error {
	return fmt.Errorf("job is already stopped")
}

func (s stoppedHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

func (s stoppedHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

func (s stoppedHandler) exited(j *Job) stateHandler {
	close(j.done)
	return stoppedHandler{}
}
