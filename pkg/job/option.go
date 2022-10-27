package job

import "github.com/rs/zerolog"

type Option func(j *Job)

// Shim is an option to set shim process binary path used to start the job.
var Shim = func(path string) Option {
	return func(j *Job) {
		j.shimPath = path
	}
}

// Cpu is an option to limit job CPU usage. Fractional, may be >1.
var Cpu = func(cpu float32) Option {
	return func(j *Job) {
		j.limits.CPU = cpu
	}
}

// Mem is an option to limit job RAM usage.
var Mem = func(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxRamBytes = bytes
	}
}

// IO is an option to limit job IO rate.
var IO = func(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxDiskIOBytes = bytes
	}
}

// UID is an option to set job process UID.
var UID = func(id int) Option {
	return func(j *Job) {
		j.ids.UID = id
	}
}

// GID is an option to set job process GID.
var GID = func(id int) Option {
	return func(j *Job) {
		j.ids.GID = id
	}
}

// cgroup is an option to override cgroup controller path.
var cgroup = func(path string) Option {
	return func(j *Job) {
		j.cgroup = path
	}
}

// dir is an option to set base jobs dir.
var dir = func(path string) Option {
	return func(j *Job) {
		j.baseJobDir = path
	}
}

// Log is an option to set job logger.
var Log = func(l zerolog.Logger) Option {
	return func(j *Job) {
		j.log = l.With().Str("id", string(j.ID)).Logger()
	}
}
