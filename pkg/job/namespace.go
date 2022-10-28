package job

import "syscall"

// remountProc remounts /proc directory to reflect PID namespace switch in a process.
func remountProc() error {
	if err := syscall.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	return syscall.Mount("proc", "/proc", "proc", 0, "")
}
