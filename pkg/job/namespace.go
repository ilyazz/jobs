package job

import "syscall"

// RemountProc remounts /proc directory to reflect PID namespace switch in a process
func RemountProc() error {
	if err := syscall.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		return err
	}
	return syscall.Mount("proc", "/proc", "proc", 0, "")
}
