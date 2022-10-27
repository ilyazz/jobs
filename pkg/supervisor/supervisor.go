package supervisor

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/ilyazz/jobs/pkg/job"
)

// ErrNotFound means the job is no longer registered
var ErrNotFound = errors.New("job not found")

type JobSupervisor struct {
	lock sync.RWMutex
	jobs map[job.ID]*job.Job
	ids  job.ExecIdentity
}

func (s *JobSupervisor) Remove(id string) error {
	s.lock.Lock()

	jid := job.ID(id)

	j, ok := s.jobs[jid]
	if !ok {
		s.lock.Unlock()
		return ErrNotFound
	}
	delete(s.jobs, jid)
	s.lock.Unlock()

	err := j.Cleanup()
	if err != nil {
		// cleanup failed. put the job back
		s.add(j)
		return err
	}

	return nil
}

func New(uid, gid int) *JobSupervisor {
	return &JobSupervisor{
		jobs: make(map[job.ID]*job.Job),
		ids: job.ExecIdentity{
			UID: uid,
			GID: gid,
		},
	}
}

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

func (s *JobSupervisor) Inspect(id string) (job.Status, int32, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return 0, 0, ErrNotFound
	}
	st, code := j.Status()

	return st, int32(code), nil
}

func (s *JobSupervisor) Logs(id string) (io.ReadCloser, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return nil, ErrNotFound
	}

	return j.Logs()
}

func (s *JobSupervisor) remove(id string) (*job.Job, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return nil, ErrNotFound
	}

	return j, nil
}

func (s *JobSupervisor) add(j *job.Job) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.jobs[j.ID] = j
}

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

func createJob(cmd string, args []string, limits job.ExecLimits, ids job.ExecIdentity) (*job.Job, error) {
	return job.New(cmd, args,
		job.Cpu(limits.CPU), job.Mem(limits.MaxRamBytes), job.IO(limits.MaxDiskIOBytes),
		job.UID(ids.UID), job.GID(ids.GID))
}

func (s *JobSupervisor) Active(id string) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	j, ok := s.jobs[job.ID(id)]
	if !ok {
		return false
	}

	return !j.Completed()
}
