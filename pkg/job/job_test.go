package job

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

func TestHappyPathShimExec(t *testing.T) {
	var cmd string
	var args []string

	uid, gid := os.Getuid(), os.Getgid()

	cgDir := t.TempDir()
	jDir := t.TempDir()

	var cmdJDir string

	startCommand = func(c *exec.Cmd) error {
		cmd = c.Path
		args = c.Args
		cmdJDir = c.Dir
		return nil
	}

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"), dir(jDir), cgroup(cgDir), Uid(uid), Gid(gid))

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

	startCommand = func(c *exec.Cmd) error {
		return nil
	}

	var waitWG sync.WaitGroup
	waitWG.Add(1)
	defer waitWG.Done()

	waitCommand = func(c *exec.Cmd) error {
		waitWG.Wait()
		return nil
	}

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"), dir(jDir), cgroup(cgDir))

	assert.NoError(t, err)
	assert.NotNil(t, j)

	st, ec := j.Status()

	assert.Equal(t, StatusActive, st)
	assert.Equal(t, -1, ec)
}

func TestHappyPathNoUidGid(t *testing.T) {
	var cmd string
	var args []string

	uid, gid := os.Getuid(), os.Getgid()

	cgDir := t.TempDir()
	jDir := t.TempDir()

	var cmdJDir string

	startCommand = func(c *exec.Cmd) error {
		cmd = c.Path
		args = c.Args
		cmdJDir = c.Dir
		return nil
	}

	j, err := New("ls", []string{"/tmp", "/var"}, Shim("/bin/shim"), dir(jDir), cgroup(cgDir))

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
