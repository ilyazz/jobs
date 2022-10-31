package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyazz/jobs/pkg/job"
	"github.com/rs/zerolog"
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
			job.Mem(31111111),
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

		//go func() {
		//	time.Sleep(2 * time.Second)
		//	j.InitStop(10 * time.Second)
		//}()

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
		//_ = j.Cleanup()

	} else {

		fmt.Printf("Running {%v %v} with PID=%v (uid:%v; gid:%v)\n", *cmd, flag.Args(), os.Getpid(), *uid, *gid)

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

		cmd := exec.Command(*cmd, flag.Args()...)
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
