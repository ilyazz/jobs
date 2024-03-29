package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/ilyazz/jobs/pkg/acl"
	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/job"
	"github.com/ilyazz/jobs/pkg/supervisor"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// JobServer implements protobuf Jobs API
type JobServer struct {
	jobs *supervisor.JobSupervisor
	auth *acl.AccessControl

	pb.UnimplementedJobServiceServer
}

// authKey is a context key to store auth subject
var authKey = struct{}{}

// StoreAuthID adds the client ID to the context object
func StoreAuthID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, authKey, id)
}

// authID retrieves the client ID from context
func authID(ctx context.Context) (string, bool) {
	v := ctx.Value(authKey)
	if v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

// Start implements API Start method
func (j *JobServer) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	cid, ok := authID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid client ID")
	}

	jid, err := j.jobs.Start(req.Command, req.Args, toJobLimits(req.Limits))
	switch {
	case errors.Is(err, supervisor.ErrNotFound):
		return nil, status.Error(codes.NotFound, "job not found")
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	}

	_ = j.auth.SetOwner(acl.ObjectID(jid), acl.UserID(cid))
	return &pb.StartResponse{
		JobId: string(jid),
	}, nil
}

// toJobLimits converts job limits object from PB to internal format
func toJobLimits(limits *pb.Limits) job.ExecLimits {
	return job.ExecLimits{
		CPU:            limits.Cpus,
		MaxDiskIOBytes: limits.Io,
		MaxRAMBytes:    limits.Memory,
	}
}

// hasReadAccess checks if user cid has read access to job jid
func (j *JobServer) hasReadAccess(cid, jid string) bool {
	return j.auth.Check(acl.AccessRequest{
		Subject: acl.UserID(cid),
		Object:  acl.ObjectID(jid),
		Action:  acl.ReadAccess,
	})
}

// hasFullAccess checks if user cid has full access to job jid
func (j *JobServer) hasFullAccess(cid, jid string) bool {
	return j.auth.Check(acl.AccessRequest{
		Subject: acl.UserID(cid),
		Object:  acl.ObjectID(jid),
		Action:  acl.FullAccess,
	})
}

// Stop implements API Stop
func (j *JobServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	cid, ok := authID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid client ID")
	}

	if !j.hasFullAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.NotFound, "job not found")
	}

	_, err := j.jobs.Stop(req.JobId, req.Mode == pb.StopMode_STOP_MODE_GRACEFUL)

	switch {
	case errors.Is(err, supervisor.ErrNotFound):
		return nil, status.Error(codes.NotFound, "job not found")
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.StopResponse{}, nil
}

// StopServer stops and cleans up all jobs
func (j *JobServer) StopServer() {
	j.jobs.StopSupervisor()
}

// Remove implements API Remove method
func (j *JobServer) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveResponse, error) {
	cid, ok := authID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid client ID")
	}

	if !j.hasFullAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.NotFound, "job not found")
	}
	err := j.jobs.Remove(req.JobId)

	switch {
	case errors.Is(err, supervisor.ErrNotFound):
		return nil, status.Error(codes.NotFound, "job not found")
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := j.auth.Remove(acl.ObjectID(req.JobId)); err != nil {
		log.Warn().Err(err).Str("id", req.JobId).Msg("failed to update ACL")
	}

	return &pb.RemoveResponse{}, nil
}

// Inspect implements GRPC Inspect method
// error is returned if:
//   - request user is not authorized for read access to the job
//   - job is not found
func (j *JobServer) Inspect(ctx context.Context, req *pb.InspectRequest) (*pb.InspectResponse, error) {

	cid, ok := authID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid client ID")
	}

	if !j.hasReadAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.NotFound, "job not found")
	}
	st, code, cmd, err := j.jobs.Inspect(req.JobId)

	switch {
	case errors.Is(err, supervisor.ErrNotFound):
		return nil, status.Error(codes.NotFound, "job not found")
	case err != nil:
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.InspectResponse{
		Details: &pb.Details{
			Command:  cmd,
			Status:   fromJobStatus(st),
			ExitCode: code,
		},
	}, nil
}

// fromJobStatus converts job status internal enum -> GRPC
func fromJobStatus(st job.Status) pb.Status {
	switch st {
	case job.StatusActive:
		return pb.Status_STATUS_ACTIVE
	case job.StatusEnded:
		return pb.Status_STATUS_ENDED
	case job.StatusStopping:
		return pb.Status_STATUS_STOPPING
	case job.StatusStopped:
		return pb.Status_STATUS_STOPPED
	default:
		return pb.Status_STATUS_UNSPECIFIED
	}
}

// Logs implements GRPC Logs method
// error is returned if:
//   - request user is not authorized for read access to the job
//   - job is not found
func (j *JobServer) Logs(req *pb.LogsRequest, server pb.JobService_LogsServer) error {

	cid, ok := authID(server.Context())
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid client ID")
	}

	if !j.hasReadAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return status.Error(codes.NotFound, "job not found")
	}

	r, err := j.jobs.Logs(req.JobId)
	switch {
	case errors.Is(err, supervisor.ErrNotFound):
		return status.Error(codes.NotFound, "job not found")
	case err != nil:
		return status.Error(codes.Internal, err.Error())
	}

	if err != nil {
		return status.Error(codes.Internal, "failed to get job output")
	}

	defer func() {
		_ = r.Close()
	}()

	data := make([]byte, 1024)

	for {
		select {
		case <-server.Context().Done():
			return status.Error(codes.Canceled, "context canceled")
		default:
		}

		n, err := r.Read(data)
		if n == 0 {
			if !req.Options.Follow {
				return nil
			}
			if !errors.Is(err, io.EOF) {
				return status.Error(codes.Internal, "failed to get job output")
			}
			if errors.Is(err, job.ErrEOFJobDone) {
				// job ended. no more output. return
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}

		err = server.Send(&pb.LogsResponse{
			Data: data[:n],
		})
		if err != nil {
			return status.Error(codes.Internal, "failed to send job output")
		}
	}
}

// New constructs a new JobServer instance
func New(cfg *Config) (*JobServer, error) {

	auth := acl.New()

	err := auth.AddSuperUsers(cfg.Superusers.FullAccess, acl.FullAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	err = auth.AddSuperUsers(cfg.Superusers.ReadAccess, acl.ReadAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %v", err)
	}

	uid, err := strconv.Atoi(cfg.IDs.UID)
	if err != nil {
		return nil, fmt.Errorf("invalid uid configured")
	}

	gid, err := strconv.Atoi(cfg.IDs.GID)
	if err != nil {
		return nil, fmt.Errorf("invalid gid configured")
	}

	rt := &JobServer{
		auth: auth,
		jobs: supervisor.New(uid, gid),
	}

	return rt, nil
}
