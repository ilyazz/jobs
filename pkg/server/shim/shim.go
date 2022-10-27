package shim

import (
	"fmt"
	"os"
)

func Main(cmd string, args []string, cpu float32, mem int64, io int64) {
	// sanity check
	if os.Args[0] != "/proc/self/exe" {
		_, _ = fmt.Fprint(os.Stderr, "should not be called directly")
		os.Exit(1)
	}

	f := os.NewFile(3, "out")
	_ = f
}

//fmt.Printf("Running {%v %v} with PID=%v (uid:%v; gid:%v)\n", *cmd, args, os.Getpid(), *uid, *gid)
/*
f := os.NewFile(3, "out")

ids := job.ExecIdentity{
UID: *uid,
GID: *gid,
}

if err := job.SetupProc(*cg, ids); err != nil {
_, _ = f.WriteString("failed to setup the process: " + err.Error())
_ = f.Close()
os.Exit(1)
}

//_ = f.Close()

if err := job.Exec(*cmd, args); err != nil {
//      fmt.Println("failed to exec: ", err)
_, _ = f.WriteString("failed to exec the process: " + err.Error())

}
*/
