package shim

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ilyazz/jobs/pkg/job"
)

func Main(command string, args []string, cgroup string, uid int, gid int) {
	// sanity check
	if os.Args[0] != "/proc/self/exe" {
		_, _ = fmt.Fprint(os.Stderr, "should not be called directly")
		os.Exit(1)
	}

	f := os.NewFile(3, "out")

	ids := job.ExecIdentity{
		UID: uid,
		GID: gid,
	}

	if err := job.SetupProc(cgroup, ids); err != nil {
		_, _ = f.WriteString("failed to setup the process: " + err.Error())
		_ = f.Close()
		os.Exit(1)
	}

	_ = f.Close()

	// must be after setting process uid/gid
	_, _, errno := syscall.RawSyscall(uintptr(syscall.SYS_PRCTL), uintptr(syscall.PR_SET_PDEATHSIG), uintptr(syscall.SIGHUP), 0)
	if errno != 0 {
		_, _ = f.WriteString("failed to setup the process")
		os.Exit(1)
	}

	hupChan := make(chan os.Signal, 1)
	signal.Notify(hupChan, syscall.SIGHUP)

	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGTERM)

	done := make(chan struct{})

	_ = f.Close()

	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to exec: %v\n", err)
	}

	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	for {
		select {
		case <-done:
			waitForOrphans()
			os.Exit(cmd.ProcessState.ExitCode())
		case <-hupChan:
			os.Exit(1)
		case s := <-termChan:
			_ = cmd.Process.Signal(s)
		}
	}
}

// waitForOrphans blocks until all child processes, including re-parented, exit
func waitForOrphans() {
	for {
		_, err := syscall.Wait4(-1, nil, 0, nil)
		if err != nil {
			// TODO: handle other than "no child processes" errors
			return
		}
	}
}
