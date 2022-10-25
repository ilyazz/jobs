package job

type Status int

const (
	// StatusActive means job is running
	StatusActive = Status(0)
	// StatusEnded means job has exited
	StatusEnded = Status(1)
	// StatusStopping means the graceful stop has been initiated, but not done yet
	StatusStopping = Status(2)
	// StatusStopped means job is stopped via API
	StatusStopped = Status(3)

	// StatusRemoved means the jobs has removed. Usually should NOT be returned anywhere
	StatusRemoved = Status(4)
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "ACTIVE"
	case StatusEnded:
		return "ENDED"
	case StatusRemoved:
		return "ZOMBIE"
	case StatusStopping:
		return "STOPPING"
	case StatusStopped:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}
