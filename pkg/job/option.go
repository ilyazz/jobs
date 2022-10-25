package job

type Option func(j *Job)

var Shim = func(path string) Option {
	return func(j *Job) {
		j.shimPath = path
	}
}

var Cpu = func(cpu float32) Option {
	return func(j *Job) {
		j.limits.CPU = cpu
	}
}

var Mem = func(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxRamBytes = bytes
	}
}

var IO = func(bytes int64) Option {
	return func(j *Job) {
		j.limits.MaxDiskIOBytes = bytes
	}
}

var Uid = func(id int) Option {
	return func(j *Job) {
		j.ids.Uid = id
	}
}

var Gid = func(id int) Option {
	return func(j *Job) {
		j.ids.Gid = id
	}
}

var cgroup = func(path string) Option {
	return func(j *Job) {
		j.cgroup = path
	}
}

var dir = func(path string) Option {
	return func(j *Job) {
		j.baseJobDir = path
	}
}
