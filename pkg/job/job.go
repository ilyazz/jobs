// //go:build linux

package job

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/spf13/afero"
)

// ExecIdentity defines user/group of job process.
type ExecIdentity struct {
	// UID is user id
	UID int
	// GID is group id
	GID int
}

// ExecLimits defines job resource limits.
type ExecLimits struct {
	CPU            float32
	MaxRamBytes    int64
	MaxDiskIOBytes int64
}

// stateHandler is an internal job state, defining how to handle job API methods.
type stateHandler interface {
	// returns current status enum value
	status() Status

	// gracefulStop inits graceful process stop
	gracefulStop(j *Job, to time.Duration) error
	// forceStop ends the job process immediately, sending SIGKILL
	forceStop(j *Job) error
	// process the process end event. send internally, when the job's j.cmd.Wait() call returns
	exited(j *Job) stateHandler
	// purge logs and working dir of the job
	cleanup(j *Job) error
	// returns a new concurrent reader object to get the job output
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

// ID is the type for Job ID.
type ID string

const defaultShimPath = "/proc/self/exe"
const defaultBasePath = "/tmp/jobs"

// Job is the main type for the job control.
type Job struct {
	// Job ID
	ID ID

	// base dir where all jobs data is located. /var/run/job by default
	baseJobDir string
	// a directory for this particular job. contains output, and the working dir
	jobDir string
	// path to the working directory
	workDir string
	// path to a binary to be used as a shim process
	shimPath string

	// wait group to control concurrent access to the output
	outLock    sync.WaitGroup
	logReaders int32

	// output file path
	outFilePath string

	// path to the cgroup controller used by the job
	cgroup string

	// job command. without args
	command string
	// job arguments
	args []string

	// Cmd object to represent the job process
	cmd *exec.Cmd

	//
	exitCode int
	done     chan struct{}

	limits ExecLimits
	ids    ExecIdentity

	stopTimer *time.Timer

	stateLock sync.Mutex
	handler   stateHandler

	log zerolog.Logger
}

// ExitCode returns (proc_exit_code, true) if the job process has ended,
// or (0, false) otherwise.
func (j *Job) ExitCode() (int, bool) {

	cp := j.cmd.ProcessState
	if cp != nil {
		return cp.ExitCode(), true
	}

	return 0, false
}

// New creates a new job to execute command 'cmd' with extra options opts.
func New(cmd string, args []string, opts ...Option) (_ *Job, reterr error) {

	j := &Job{
		ID: ID(xid.New().String()),

		done:       make(chan struct{}),
		shimPath:   defaultShimPath,
		baseJobDir: defaultBasePath,
		ids: ExecIdentity{
			UID: os.Getuid(),
			GID: os.Getgid(),
		},
		log: zerolog.New(io.Discard), // do not log by default
	}

	for _, o := range opts {
		o(j)
	}

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
			j.log.Warn().Err(reterr).Msg("failed to start job")
			if of != nil {
				_ = of.Close()
			}
			if err := j.rmJobDirs(); err != nil {
				j.log.Warn().Err(err).Msg("failed to undo")
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

	j.cmd.Stdout = of
	j.cmd.Stderr = of

	j.cmd.ExtraFiles = append(j.cmd.ExtraFiles, w)
	j.cmd.Dir = j.workDir

	j.cmd.SysProcAttr = &syscall.SysProcAttr{
		// new net and mount namespaces
		Unshareflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
		// new PID namespace
		Cloneflags: syscall.CLONE_NEWPID,
	}

	j.log.Info().Str("jid", string(j.ID)).Msgf("start proc for: %q %v", j.cmd.Path, j.cmd.Args)

	if err := startCommand(j.cmd); err != nil {
		return nil, err
	}

	// need to close the local copy of write-end to receive io.EOF when the child does the same
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
		j.log.Info().Int("exit_code", j.cmd.ProcessState.ExitCode()).Msg("job ended")

		j.exited()
	}()

	return j, nil
}

func (j *Job) rmJobDirs() error {
	return os.RemoveAll(j.jobDir)
}

func (j *Job) initJobDirs() error {

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
			j.log.Warn().Err(err2).Msg("failed to undo")
		}
		return err
	}

	// Working dir needs to be accessible by user
	if err := appFs.Chown(wd, j.ids.UID, j.ids.GID); err != nil {
		if err2 := appFs.RemoveAll(jobDir); err2 != nil {
			j.log.Warn().Err(err2).Msg("failed to undo")
		}
		return err
	}

	j.jobDir = jobDir
	j.workDir = wd
	j.outFilePath = filepath.Join(out, "output")

	return nil
}

// cmdArgs creates a string slice of arguments to be passed to the shim process.
func (j *Job) cmdArgs() []string {
	rt := []string{"--mode=shim",
		fmt.Sprintf("--cmd=%s", j.command),
		fmt.Sprintf("--cgroup=%s", j.cgroup),
		fmt.Sprintf("--uid=%d", j.ids.UID),
		fmt.Sprintf("--gid=%d", j.ids.GID),
	}

	if len(j.args) > 0 {
		rt = append(rt, "--")
		rt = append(rt, j.args...)
	}

	return rt
}

// Status returns the job current status, and exit code, if it's ended or stopped. If not, exit code is 0.
func (j *Job) Status() (Status, int) {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()
	st := j.handler.status()
	if st == StatusActive || st == StatusStopping {
		return st, 0
	}
	return st, j.cmd.ProcessState.ExitCode()
}

// Completed returns if the job process is still running and additional output can be produced
func (j *Job) Completed() bool {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	st := j.handler.status()
	return st != StatusActive && st != StatusStopping
}

// Status returns the job current status, and exit code, if it's ended or stopped. If not, exit code is 0.
func (j *Job) setHandler(h stateHandler) {
	j.log.Debug().Msgf("change job state %s -> %s", j.handler.status(), h.status())
	j.handler = h
}

// InitStop starts "Graceful Stop", sending initial stop signal, and starting timer to send SIGKILL.
func (j *Job) InitStop(to time.Duration) error {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	err := j.handler.gracefulStop(j, to)
	if err == nil {
		j.setHandler(stoppingHandler{})
	}

	return err
}

// Stop ends the job process.
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
// Waits for all active log readers to close.
func (j *Job) Cleanup() error {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	err := j.handler.cleanup(j)
	if err == nil {
		j.setHandler(zombieHandler{})
	}
	j.log.Info().Msg("job artifacts removed")

	return err
}

// Logs creates a new Reader object to provide job output
// should be called under state lock.
func (j *Job) Logs() (io.ReadCloser, error) {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	r, err := j.handler.logs(j)
	if err == nil {
		n := atomic.AddInt32(&j.logReaders, 1)
		j.log.Info().Int32("total", n).Msg("log reader added")
	}

	return r, err
}

// exited is used internally to update the job state when the job process ended.
func (j *Job) exited() {
	j.stateLock.Lock()
	defer j.stateLock.Unlock()

	ps := j.cmd.ProcessState
	if ps != nil {
		j.exitCode = ps.ExitCode()
	}

	if err := j.removeCgroup(); err != nil {
		j.log.Warn().Err(err).Msg("failed to delete cgroup")
	}

	j.setHandler(j.handler.exited(j))
}

// logsReader is an internal method that does all the Logs() actual work.
func (j *Job) logsReader() (io.ReadCloser, error) {
	f, err := appFs.OpenFile(j.outFilePath, os.O_RDONLY, 0200)
	if err != nil {
		return nil, fmt.Errorf("failed to get output: %w", err)
	}

	r := outputReader{
		f:       f,
		lock:    &j.outLock,
		counter: &j.logReaders,
	}

	j.outLock.Add(1)

	return &r, nil
}

// doCleanup waits for all readers to complete and then removes the job directory
func (j *Job) doCleanup() error {
	// supposed to be called under j.stateLock
	// wait for all Log readers to close
	j.log.Debug().Int32("total", j.logReaders).Msg("cleanup: waiting for log readers...")
	j.outLock.Wait()
	j.log.Debug().Msg("cleanup: all log readers closed.")

	return appFs.RemoveAll(j.jobDir)
}

// sendStopSignal sends a signal to the job process to init stop.
func (j *Job) sendStopSignal(graceful bool) error {
	// supposed to be called under j.stateLock
	s := syscall.SIGKILL
	if graceful {
		s = syscall.SIGTERM
	}
	return signalCommand(j.cmd, s)
}

// startCommand is just a wrapper around exec.Command.Start. for mocks.
var startCommand = func(c *exec.Cmd) error {
	return c.Start()
}

// startCommand is just a wrapper around exec.Command.Wait. for mocks.
var waitCommand = func(c *exec.Cmd) error {
	return c.Wait()
}

// signalCommand is just a wrapper around exec.Command.Signal. for mocks.
var signalCommand = func(c *exec.Cmd, s os.Signal) error {
	return c.Process.Signal(s)
}

// to be able to mock 'exec' syscall in tests.
var sysExec = syscall.Exec

type sysStub interface {
	Exec(argv0 string, argv []string, envv []string) error
	Start(c *exec.Cmd) error
	Wait(c *exec.Cmd) error
	Signal(c *exec.Cmd, s os.Signal) error
}

// appFs is a wrapper around FS operations. for mocks.
var appFs = afero.NewOsFs()

// Exec replaces the current process with cmd
// supposed to be called from a shim process.
func Exec(cmd string, args []string) error {
	pcmd, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("%q not found: %w", cmd, err)
	}

	return sysExec(pcmd, args, os.Environ())
}

// Wait waits until the job state goes to Ended or Stopped.
func (j *Job) Wait() {
	<-j.done
}
