package main

import (
	"flag"
	"fmt"
	"github.com/ilyazz/jobs/pkg/job"
	"github.com/rs/zerolog"
	"os"
	"time"
)

var mode = flag.String("mode", "", "binary mode")
var cmd = flag.String("cmd", "", "do not run directly")
var cg = flag.String("cgroup", "", "do not run directly")

var uid = flag.Int("uid", 0, "do not run directly")
var gid = flag.Int("gid", 0, "do not run directly")

func main() {

	if os.Getuid() != 0 {
		_, _ = fmt.Fprintln(os.Stderr, "Please run as root")
		os.Exit(1)
	}

	flag.Parse()

	if *mode != "shim" {
		cmd := flag.Args()
		j, err := job.New(cmd[0], cmd[1:],
			job.Shim("/proc/self/exe"),
			job.UID(1000),
			job.GID(1000),
			job.Log(zerolog.New(os.Stdout).With().Timestamp().Logger()))
		if err != nil {
			panic(err)
		}

		fmt.Printf("Started job #%s\n", j.ID)

		r, err := j.Logs()
		if err != nil {
			panic(err)
		}

		r2, err := j.Logs()
		if err != nil {
			panic(err)
		}

		go func() {
			time.Sleep(2 * time.Second)
			_ = r2.Close()
		}()

		for {
			data := make([]byte, 64)
			n, _ := r.Read(data)
			if n == 0 {
				if !j.Completed() {
					time.Sleep(250 * time.Millisecond)
					continue
				}
				break
			}
			fmt.Print(string(data[:n]))
		}

		_ = r.Close()
		_ = j.Cleanup()

	} else {

		args := []string{*cmd}

		args = append(args, flag.Args()...)

		fmt.Printf("Running {%v %v} with PID=%v (uid:%v; gid:%v)\n", *cmd, args, os.Getpid(), *uid, *gid)

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
			//	fmt.Println("failed to exec: ", err)
			_, _ = f.WriteString("failed to exec the process: " + err.Error())
			os.Exit(-1)
		}
	}
}
