package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/ilyazz/jobs/pkg/acl"
	"github.com/ilyazz/jobs/pkg/job"
	"github.com/ilyazz/jobs/pkg/supervisor"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
)

// JobServer implements protobuf Jobs API
type JobServer struct {
	jobs *supervisor.JobSupervisor
	auth *acl.AccessControl

	pb.UnimplementedJobServiceServer
}

// clientID extracts a client ID from GRPC context
func clientID(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("no client ID")
	}
	tlsInfo := p.AuthInfo.(credentials.TLSInfo)
	if (len(tlsInfo.State.VerifiedChains) < 1) ||
		(len(tlsInfo.State.VerifiedChains[0]) < 1) {
		return "", fmt.Errorf("no DN provided")
	}

	subj := tlsInfo.State.VerifiedChains[0][0].Subject
	return subj.String(), nil
}

// Start implements API Start method
func (j *JobServer) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {

	cid, err := clientID(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid client ID")
	}

	jid, err := j.jobs.Start(req.Command, req.Args, toJobLimits(req.Limits))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to start job")
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
		MaxRamBytes:    limits.Memory,
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
	cid, err := clientID(ctx)
	if err != nil {
		return nil, err
	}

	if !j.hasFullAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.Internal, "job not found")
	}

	_, err = j.jobs.Stop(req.JobId, req.Mode == pb.StopMode_STOP_MODE_GRACEFUL)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to stop the job")
	}
	return &pb.StopResponse{}, nil
}

// StopServer stops and cleans up all jobs
func (j *JobServer) StopServer() {
	j.jobs.StopSupervisor()
}

// Remove implements API Remove method
func (j *JobServer) Remove(ctx context.Context, req *pb.RemoveRequest) (*pb.RemoveResponse, error) {
	cid, err := clientID(ctx)
	if err != nil {
		return nil, err
	}

	if !j.hasFullAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.Internal, "job not found")
	}
	return &pb.RemoveResponse{}, j.jobs.Remove(req.JobId)
}

// Inspect implements API Inspect method
func (j *JobServer) Inspect(ctx context.Context, req *pb.InspectRequest) (*pb.InspectResponse, error) {

	cid, err := clientID(ctx)
	if err != nil {
		return nil, err
	}

	if !j.hasReadAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return nil, status.Error(codes.Internal, "job not found")
	}
	st, code, err := j.jobs.Inspect(req.JobId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get job details")
	}

	return &pb.InspectResponse{
		Details: &pb.Details{
			Status:   fromJobStatus(st),
			ExitCode: code,
		},
	}, nil
}

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

func (j *JobServer) Logs(req *pb.LogsRequest, server pb.JobService_LogsServer) error {

	cid, err := clientID(server.Context())
	if err != nil {
		return err
	}

	if !j.hasReadAccess(cid, req.JobId) {
		log.Info().Str("client", cid).Str("job", req.JobId).Msg("no access")
		return status.Error(codes.Internal, "job not found")
	}

	r, err := j.jobs.Logs(req.JobId)
	if err != nil {
		return status.Error(codes.Internal, "failed to get job output")
	}

	defer func() {
		_ = r.Close()
	}()

	data := make([]byte, 1024)
	// read data until available
	for {
		n, err := r.Read(data)
		if n == 0 {
			if !req.Options.Follow {
				return nil
			}
			if !errors.Is(err, io.EOF) {
				return status.Error(codes.Internal, "failed to get job output")
			}
			if j.jobs.Active(req.JobId) {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			// job ended. no more output. return
			return nil
		}

		err = server.Send(&pb.LogsResponse{
			Data: data[:n],
		})
		if err != nil {
			return status.Error(codes.Internal, "failed to send job output")
		}
	}
}

func (j *JobServer) mustEmbedUnimplementedJobServiceServer() {
	panic("implement me")
}

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
