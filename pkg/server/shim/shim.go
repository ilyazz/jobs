package shim

import (
	"fmt"
	"os"

	"github.com/ilyazz/jobs/pkg/job"
)

func Main(cmd string, args []string, cgroup string, uid int, gid int) {
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

	args = append([]string{cmd}, args...)

	_ = f.Close()
	if err := job.Exec(cmd, args); err != nil {
		_, _ = f.WriteString("failed to exec the process: " + err.Error())
		os.Exit(1)
	}
}
