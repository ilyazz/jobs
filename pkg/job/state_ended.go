package job

import (
	"fmt"
	"io"
	"time"
)

func (e endedHandler) status() Status {
	return StatusEnded
}

func (e endedHandler) gracefulStop(*Job, time.Duration) error {
	return fmt.Errorf("job already ended")
}

func (e endedHandler) forceStop(*Job) error {
	return fmt.Errorf("job already ended")
}

func (e endedHandler) cleanup(j *Job) error {
	return j.doCleanup()
}

func (e endedHandler) logs(j *Job) (io.ReadCloser, error) {
	return j.logsReader()
}

func (e endedHandler) exited(*Job) stateHandler {
	// should never happen
	return endedHandler{}
}
