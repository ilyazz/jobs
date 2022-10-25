package job

import (
	"fmt"
	"io"
	"time"
)

func (z zombieHandler) status() Status {
	// should never happen
	return StatusRemoved
}

func (z zombieHandler) forceStop(*Job) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

func (z zombieHandler) gracefulStop(*Job, time.Duration) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

func (z zombieHandler) cleanup(*Job) error {
	// should never happen
	return fmt.Errorf("job is removed")
}

func (z zombieHandler) logs(*Job) (io.ReadCloser, error) {
	// should never happen
	return nil, fmt.Errorf("job is removed")
}

func (z zombieHandler) exited(*Job) stateHandler {
	// should never happen
	return zombieHandler{}
}
