//go:build linux

package job

import (
	"fmt"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var appFs = afero.NewOsFs()

// ExecIdentity defines user/group of job process
type ExecIdentity struct {
	// Uid is user id
	Uid int
	// Gid is group id
	Gid int
}

// ExecLimits defines job resource limits
type ExecLimits struct {
	CPU            float32
	MaxRamBytes    int64
	MaxDiskIOBytes int64
}

// stateHandler is an internal job state, defining how to handle job API methods
type stateHandler interface {
	status() Status

	gracefulStop(j *Job, to time.Duration) error
	forceStop(j *Job) error
	exited(j *Job) stateHandler

	cleanup(j *Job) error

	logs(j *Job) (io.ReadCloser, error)
}

type activeHandler struct{}
type endedHandler struct{}
type stoppingHandler struct{}
type stoppedHandler struct{}
type zombieHandler struct{}

// make sure all handlers implement stateHandler interface
var _ stateHandler = activeHandler{}
var _ stateHandler = endedHandler{}
var _ stateHandler = stoppingHandler{}
var _ stateHandler = stoppedHandler{}
var _ stateHandler = zombieHandler{}

type ID string

type Config struct {
	ShimPath string
}

const defaultShimPath = "/proc/self/exe"

type Job struct {
	ID ID

	baseJobDir string
	jobDir     string

	workDir  string
	shimPath string

	outLock     sync.WaitGroup
	outFilePath string

	cgroup string

	command string
	args    []string

	cmd *exec.Cmd

	exitCode int
	done     atomic.Bool

	limits ExecLimits
	ids    ExecIdentity

	stopTimer *time.Timer
	stateLock sync.Mutex
	handler   stateHandler
}

// ExitCode returns (proc_exit_code, true) if the job process has ended,
// or (0, false) otherwise
func (j *Job) ExitCode() (int, bool) {

	cp := j.cmd.ProcessState
	if cp != nil {
		return cp.ExitCode(), true
	}

	return 0, false
}

var startCommand = func(c *exec.Cmd) error {
	return c.Start()
}

var waitCommand = func(c *exec.Cmd) error {
	return c.Wait()
}

// New creates a new job to execute command 'cmd' with extra options opts
func New(cmd string, args []string, opts ...Option) (_ *Job, reterr error) {

	j := &Job{
		shimPath: defaultShimPath,
		ids: ExecIdentity{
			Uid: os.Getuid(),
			Gid: os.Getgid(),
		},
	}

	for _, o := range opts {
		o(j)
	}

	j.ID = ID(xid.New().String())

	j.command = cmd
	j.args = args

	if err := j.initJobDirs(); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	j.handler = activeHandler{}

	var of afero.File
	var cgPath string

	defer func() {
		// if something is wrong, and we return an error from New(..) - remove the job dir
		if reterr != nil {
			if of != nil {
				_ = of.Close()
			}
			if err := j.rmJobDirs(); err != nil {
				log.Warn().Err(err).Msg("failed to undo")
			}
		}
		if cgPath != "" {
			_ = appFs.Remove(cgPath)
		}
	}()

	of, err := appFs.Create(j.outFilePath)
	if err != nil {
		return nil, err
	}

	cgPath, err = j.setupCgroup()
	if err != nil {
		return nil, err
	}

	j.cgroup = cgPath

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	defer func() { _ = r.Close() }()

	j.cmd = exec.Command(j.shimPath, j.cmdArgs()...)

	j.cmd.Stdout = io.MultiWriter(of, os.Stdout)
	j.cmd.Stderr = io.MultiWriter(of, os.Stdout)
	j.cmd.ExtraFiles = append(j.cmd.ExtraFiles, w)
	j.cmd.Dir = j.workDir

	j.cmd.SysProcAttr = &syscall.SysProcAttr{
		// new net and mount namespaces
		Unshareflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
		// new PID namespace
		Cloneflags: syscall.CLONE_NEWPID,
	}

	if err := startCommand(j.cmd); err != nil {
		return nil, err
	}

	// need to close local copy of write-end to receive io.EOF when the child does the same
	// but cannot do it before cmd.Start() returns
	_ = w.Close()

	childMsg, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	if len(childMsg) > 0 {
		return nil, fmt.Errorf("failed to start the process: %q", string(childMsg))
	}

	go func() {
		defer func() { _ = of.Close() }()
		_ = waitCommand(j.cmd)
		j.exited()
	}()

	return j, nil
}

const defBasePath = "/var/run/jobs/"

func (j *Job) rmJobDirs() error {
	return os.RemoveAll(j.jobDir)
}

func (j *Job) initJobDirs() error {

	if j.baseJobDir == "" {
		j.baseJobDir = defBasePath
	}
	jobDir := filepath.Join(j.baseJobDir, string(j.ID))

	// purge, if the working dir already exists
	_ = appFs.RemoveAll(jobDir)

	out := filepath.Join(jobDir, "out")
	if err := appFs.MkdirAll(out, 0700); err != nil {
		return err
	}

	wd := filepath.Join(jobDir, "workDir")
	if err := appFs.MkdirAll(wd, 0700); err != nil {
		if err2 := appFs.RemoveAll(jobDir); err2 != nil {
			log.Warn().Err(err2).Msg("failed to undo")
		}
		return err
	}

	// Working dir needs to be accessible by user
	if err := appFs.Chown(wd, j.ids.Uid, j.ids.Gid); err != nil {
		if err2 := appFs.RemoveAll(jobDir); err2 != nil {
			log.Warn().Err(err2).Msg("failed to undo")
		}
		return err
	}

	j.jobDir = jobDir
	j.workDir = wd
	j.outFilePath = filepath.Join(out, "output")

	return nil
}

// cmdArgs creates a string slice of arguments to be passed to the shim process
func (j *Job) cmdArgs() []string {
	rt := []string{"--mode=shim",
		fmt.Sprintf("--cmd=%s", j.command),
		fmt.Sprintf("--cgroup=%s", j.cgroup),
		fmt.Sprintf("--uid=%d", j.ids.Uid),
		fmt.Sprintf("--gid=%d", j.ids.Gid),
	}

	if len(j.args) > 0 {
		rt = append(rt, "--")
		rt = append(rt, j.args...)
	}

	return rt
}

func (j *Job) Status() (Status, int) {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	return j.handler.status(), j.cmd.ProcessState.ExitCode()
}

func (j *Job) setHandler(h stateHandler) {
	fmt.Printf("Change job state %s -> %s\n", j.handler.status(), h.status())
	j.handler = h
}

// InitStop starts "Graceful Stop", sending initial stop signal, and starting timer to send SIGKILL
func (j *Job) InitStop(to time.Duration) error {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	err := j.handler.gracefulStop(j, to)
	if err == nil {
		j.setHandler(stoppingHandler{})
	}

	return err
}

// Stop ends the job process
func (j *Job) Stop() error {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	err := j.handler.forceStop(j)
	if err == nil {
		j.setHandler(stoppedHandler{})
	}
	return err

}

// Cleanup purges the stopped/ended job, remove all the logs and files in working directory
func (j *Job) Cleanup() error {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	err := j.handler.cleanup(j)
	if err == nil {
		j.setHandler(zombieHandler{})
	}

	return err
}

// Logs creates a new Reader object to provide job output
// should be called under state lock
func (j *Job) Logs() (io.ReadCloser, error) {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	return j.handler.logs(j)
}

// exited is used internally to update the job state when the job process ended
func (j *Job) exited() {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	j.done.Store(true)
	ps := j.cmd.ProcessState
	if ps != nil {
		j.exitCode = ps.ExitCode()
	}

	j.setHandler(j.handler.exited(j))
}

// logsReader is an internal method that does all the Logs() actual work
func (j *Job) logsReader() (io.ReadCloser, error) {
	f, err := appFs.OpenFile(j.outFilePath, os.O_RDONLY, 0200)
	if err != nil {
		return nil, fmt.Errorf("failed to get output: %w", err)
	}

	r := outputReader{
		f:    f,
		lock: &j.outLock,
	}

	j.outLock.Add(1)

	return &r, nil
}

// doCleanup waits for all readers to complete and then removes the job directory
func (j *Job) doCleanup() error {
	// supposed to be called under j.stateLock
	// wait for all log readers to close
	j.outLock.Wait()
	return appFs.Remove(j.jobDir)
}

// sendStopSignal sends a signal to the job process to init stop
func (j *Job) sendStopSignal(graceful bool) error {
	// supposed to be called under j.stateLock
	s := syscall.SIGKILL
	if graceful {
		s = syscall.SIGTERM
	}
	return j.cmd.Process.Signal(s)
}

// to be able to mock 'exec' syscall in tests
var sysExec = syscall.Exec

// Exec replaces the current process with cmd
// supposed to be called from a shim process
func Exec(cmd string, args []string) error {
	pcmd, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("%q not found: %w", cmd, err)
	}

	return sysExec(pcmd, args, os.Environ())
}
