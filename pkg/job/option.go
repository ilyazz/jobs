package job

import (
	"os"
	"os/exec"

	"github.com/rs/zerolog"
)

type Option func(j *Job)

// Shim is an option to set shim process binary path used to start the job.
func Shim(path string) Option {
	return func(j *Job) {
		j.shimPath = path
	}
}

// Cpu is an option to limit job CPU usage. Fractional, may be >1.
func CPU(cpu float32) Option {
	return func(j *Job) {
		j.limits.CPU = cpu
	}
}

// Mem is an option to limit job RAM usage.
func Mem(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxRAMBytes = bytes
	}
}

// IO is an option to limit job IO rate.
func IO(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxDiskIOBytes = bytes
	}
}

// UID is an option to set job process UID.
func UID(id int) Option {
	return func(j *Job) {
		j.ids.UID = id
	}
}

// GID is an option to set job process GID.
func GID(id int) Option {
	return func(j *Job) {
		j.ids.GID = id
	}
}

// cgroup is an option to override cgroup controller path.
func cgroup(path string) Option {
	return func(j *Job) {
		j.cgroupOuter = path
	}
}

// dir is an option to set base job dir.
func dir(path string) Option {
	return func(j *Job) {
		j.baseJobDir = path
	}
}

// cmdStart is an option to mock start operation
func cmdStart(start func(c *exec.Cmd) error) Option {
	return func(j *Job) {
		j.syscalls.start = start
	}
}

// cmdWait is an option to mock wait operation
func cmdWait(wait func(c *exec.Cmd) error) Option {
	return func(j *Job) {
		j.syscalls.wait = wait
	}
}

// cmdSignal is an option to mock signal operation
func cmdSignal(signal func(c *exec.Cmd, s os.Signal) error) Option {
	return func(j *Job) {
		j.syscalls.signal = signal
	}
}

// Log is an option to set job logger.
func Log(l zerolog.Logger) Option {
	return func(j *Job) {
		j.log = l.With().Str("id", string(j.ID)).Logger()
	}
}
