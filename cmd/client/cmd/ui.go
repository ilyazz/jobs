package cmd

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// diagMessage converts GRPC error to user-friendly string message
func diagMessage(err error) string {

	gerr, ok := status.FromError(err)
	if ok {
		switch gerr.Code() {
		case codes.Internal:
			return err.Error()
		case codes.NotFound:
			return "no such job"
		case codes.Unauthenticated:
			return "invalid certificate"
		}
	}

	return err.Error()
}
