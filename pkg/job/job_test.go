package job

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var lg = zerolog.New(os.Stdout).With().Logger()

func TestHappyPathShimExec(t *testing.T) {
	var cmd string
	var args []string

	uid, gid := os.Getuid(), os.Getgid()

	cgOutDir := t.TempDir()
	cgDir := filepath.Join(cgOutDir, "inner")

	jDir := t.TempDir()

	var cmdJDir string

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"),
		dir(jDir), cgroup(cgOutDir),
		cmdStart(func(c *exec.Cmd) error {
			cmd = c.Path
			args = c.Args
			cmdJDir = c.Dir
			return nil
		}),
		UID(uid), GID(gid), Log(lg))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	assert.Equal(t, filepath.Join(jDir, string(j.ID), "workDir"), cmdJDir)

	assert.Equal(t, "/bin/shim", cmd)
	assert.Equal(t, []string{"/bin/shim", "--mode=shim", "--cmd=ls",
		"--cgroup=" + cgDir,
		fmt.Sprintf("--uid=%d", uid),
		fmt.Sprintf("--gid=%d", gid),
		"--", "/tmp", "/var"}, args)

}

func TestHappyPathStatus(t *testing.T) {

	cgDir := t.TempDir()
	jDir := t.TempDir()

	var waitWG sync.WaitGroup
	waitWG.Add(1)
	defer waitWG.Done()

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"),
		cmdStart(defStart),
		cmdWait(func(c *exec.Cmd) error {
			waitWG.Wait()
			return nil
		}),
		dir(jDir), cgroup(cgDir))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	st, ec := j.Status()

	assert.Equal(t, StatusActive, st)
	assert.Equal(t, 0, ec)
}

func TestHappyPathNoUidGid(t *testing.T) {
	var cmd string
	var args []string

	uid, gid := os.Getuid(), os.Getgid()

	cgOutDir := t.TempDir()
	cgDir := filepath.Join(cgOutDir, "inner")

	jDir := t.TempDir()

	var cmdJDir string

	j, err := New("ls", []string{"/tmp", "/var"},
		Shim("/bin/shim"),
		cmdStart(func(c *exec.Cmd) error {
			cmd = c.Path
			args = c.Args
			cmdJDir = c.Dir
			return nil
		}),
		dir(jDir), cgroup(cgOutDir))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	assert.Equal(t, filepath.Join(jDir, string(j.ID), "workDir"), cmdJDir)

	assert.Equal(t, "/bin/shim", cmd)
	assert.Equal(t, []string{"/bin/shim", "--mode=shim", "--cmd=ls",
		"--cgroup=" + cgDir,
		fmt.Sprintf("--uid=%d", uid),
		fmt.Sprintf("--gid=%d", gid),
		"--", "/tmp", "/var"}, args)

}

func defStart(c *exec.Cmd) error {
	return nil
}

func defWait(c *exec.Cmd) error {
	return nil
}

func TestCgroupConfig(t *testing.T) {
	cgDir := t.TempDir()
	jDir := t.TempDir()

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/true"),
		dir(jDir), cgroup(cgDir),
		cmdStart(defStart), cmdWait(defWait),
		Log(lg), Cpu(3.14), Mem(27), IO(34))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	// verify cgroup cpu
	cpuCg, err := os.ReadFile(filepath.Join(cgDir, "inner", "cpu.max"))
	assert.NoError(t, err)
	assert.Equal(t, "31400.0020 10000.0000\n", string(cpuCg))

	// verify cgroup memory
	memCg, err := os.ReadFile(filepath.Join(cgDir, "inner", "memory.max"))
	assert.NoError(t, err)
	assert.Equal(t, "27\n", string(memCg))

	// verify cgroup IO
	ioCg, err := os.ReadFile(filepath.Join(cgDir, "inner", "io.max"))
	assert.NoError(t, err)

	s := bufio.NewScanner(bytes.NewReader(ioCg))
	for s.Scan() {
		ss := strings.SplitN(s.Text(), " ", 2)
		assert.Equal(t, "rbps=34 wbps=34", ss[1])
	}
}

func TestStop(t *testing.T) {
	cgDir := t.TempDir()
	jDir := t.TempDir()

	var jend sync.WaitGroup
	jend.Add(1)

	var s os.Signal
	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"),
		dir(jDir), cgroup(cgDir),
		cmdStart(defStart),
		cmdWait(func(c *exec.Cmd) error {
			jend.Wait()
			return nil
		}),
		cmdSignal(func(c *exec.Cmd, ts os.Signal) error {
			s = ts
			return nil
		}),
		Log(lg))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	err = j.Stop()
	assert.NoError(t, err)

	st, _ := j.Status()
	assert.Equal(t, StatusStopped, st)
	assert.Equal(t, syscall.SIGKILL, s)

	jend.Done()
	st, _ = j.Status()
	assert.Equal(t, StatusStopped, st)
}

func TestGracefulStop(t *testing.T) {
	cgDir := t.TempDir()
	jDir := t.TempDir()

	var jend sync.WaitGroup
	jend.Add(1)

	var s os.Signal

	j, err := New("ls", []string{"/tmp", "/var"},
		Shim("/bin/true"),
		cmdStart(defStart),
		cmdWait(func(c *exec.Cmd) error {
			jend.Wait()
			return nil
		}),
		cmdSignal(func(c *exec.Cmd, ts os.Signal) error {
			s = ts
			return nil
		}),
		dir(jDir), cgroup(cgDir), Log(lg))
	assert.NoError(t, err)
	assert.NotNil(t, j)

	err = j.InitStop(time.Hour)
	assert.NoError(t, err)

	st, _ := j.Status()
	assert.Equal(t, StatusStopping, st)
	assert.Equal(t, syscall.SIGTERM, s)

	jend.Done()
	j.Wait()

	st, _ = j.Status()
	assert.Equal(t, StatusStopped, st)
}

func TestLogs(t *testing.T) {
	cgDir := t.TempDir()
	jDir := t.TempDir()

	var jend sync.WaitGroup
	jend.Add(1)
	j, err := New("ls", []string{"/tmp", "/var"},
		Shim("/bin/shim"),
		cmdStart(defStart),
		cmdWait(func(c *exec.Cmd) error {
			jend.Wait()
			return nil
		}),
		cmdSignal(func(c *exec.Cmd, ts os.Signal) error {
			return nil
		}),
		Log(lg), dir(jDir), cgroup(cgDir))
	assert.NoError(t, err)
	assert.NotNil(t, j)

	_ = j.Stop()

	jend.Done()
	j.Wait()

	l1, err := j.Logs()
	assert.NoError(t, err)

	l2, err := j.Logs()
	assert.NoError(t, err)

	st, _ := j.Status()
	assert.Equal(t, StatusStopped, st)

	_ = l1.Close()
	st, _ = j.Status()
	assert.Equal(t, StatusStopped, st)

	_ = l2.Close()
	st, _ = j.Status()
	assert.Equal(t, StatusStopped, st)

	err = j.Cleanup()
	assert.NoError(t, err)
}
