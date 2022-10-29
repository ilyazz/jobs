package supervisor

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ilyazz/jobs/pkg/job"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ErrNotFound means the job is no longer registered
var ErrNotFound = errors.New("job not found")

// JobSupervisor manages the jobs
type JobSupervisor struct {
	lock sync.RWMutex
	jobs map[job.ID]*job.Job

	// ids - uid/gid used by supervisor to run job processes
	ids job.ExecIdentity
}

// Remove all job artifacts, and the unlinks the job id from supervisor
func (s *JobSupervisor) Remove(id string) error {
	s.lock.Lock()
	// intentionally no defer unlock. see comment below
	jid := job.ID(id)

	j, ok := s.jobs[jid]
	if !ok {
		s.lock.Unlock()
		return ErrNotFound
	}
	if !j.Completed() {
		s.lock.Unlock()
		return fmt.Errorf("job is still running")
	}

	delete(s.jobs, jid)
	// j.Cleanup() can take a while, don't block other operations and release the lock here
	s.lock.Unlock()

	err := j.Cleanup()
	if err != nil {
		// cleanup failed. put the job back
		s.add(j)
		return err
	}

	return nil
}

// New creates s new job supervisor. All jobs will be run with uid/gid credentials
func New(uid, gid int) *JobSupervisor {
	return &JobSupervisor{
		jobs: make(map[job.ID]*job.Job),
		ids: job.ExecIdentity{
			UID: uid,
			GID: gid,
		},
	}
}

// Start a new job with given parameters
func (s *JobSupervisor) Start(cmd string, args []string, limits job.ExecLimits) (job.ID, error) {
	j, err := createJob(cmd, args, limits, s.ids)
	if err != nil {
		log.Warn().Err(err).Str("cmd", cmd).Msg("failed to start the job")
		return "", err
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.jobs[j.ID] = j

	return j.ID, nil
}

// StopSupervisor ends all current jobs
// first, it initiates graceful stop with 10 sec timeout
// then it wait for all jobs to stop, and cleans them up
func (s *JobSupervisor) StopSupervisor() {
	s.lock.Lock()
	defer s.lock.Unlock()

	var wg sync.WaitGroup
	for _, j := range s.jobs {
		_ = j.InitStop(10 * time.Second)

		wg.Add(1)
		go func(j *job.Job) {
			j.Wait()
			wg.Done()
		}(j)
	}
	wg.Wait()
	for _, j := range s.jobs {
		_ = j.Cleanup()
	}
}

func (s *JobSupervisor) Stop(id string, graceful bool) (any, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return nil, ErrNotFound
	}

	var err error
	if graceful {
		err = j.InitStop(30 * time.Second)
	} else {
		err = j.Stop()
	}
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("failed to stop the job")
	}

	return nil, err
}

// Inspect returns job details: status, exit code and command
// TODO get rid of 4 ret values, add a structure
func (s *JobSupervisor) Inspect(id string) (job.Status, int32, string, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return 0, 0, "", ErrNotFound
	}
	st, code := j.Status()

	cmd := append([]string{j.Command}, j.Args...)
	return st, int32(code), strings.Join(cmd, " "), nil
}

// Logs returns log reader for job id
func (s *JobSupervisor) Logs(id string) (io.ReadCloser, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return nil, ErrNotFound
	}

	return j.Logs()
}

// remove deletes the job id from internal storage
func (s *JobSupervisor) remove(id string) (*job.Job, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return nil, ErrNotFound
	}

	return j, nil
}

// add the job id to internal storage
func (s *JobSupervisor) add(j *job.Job) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.jobs[j.ID] = j
}

// Cleanup does cleanup for job id and unlinks it
func (s *JobSupervisor) Cleanup(id string) error {
	j, err := s.remove(id)
	if err != nil {
		return err
	}

	err = j.Cleanup()
	if err != nil {
		s.add(j)
		return err
	}

	return nil
}

// createJob is an internal wrapper for job.New(..)
func createJob(cmd string, args []string, limits job.ExecLimits, ids job.ExecIdentity) (*job.Job, error) {
	return job.New(cmd, args,
		job.CPU(limits.CPU), job.Mem(limits.MaxRAMBytes), job.IO(limits.MaxDiskIOBytes),
		job.UID(ids.UID), job.GID(ids.GID),
		job.Log(zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})))
}
