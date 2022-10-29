package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/client"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the remote job",
	Long:  `Stop the remote job`,
	Run: func(cmd *cobra.Command, args []string) {
		_, cfg, err := client.FindConfig(config)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to load config\n")
			os.Exit(1)
		}

		if len(args) == 0 {
			_, _ = fmt.Fprintf(os.Stderr, "job_id required\n")
			os.Exit(1)
		}

		cl, err := client.New(cfg)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
			os.Exit(1)
		}

		stopMode := pb.StopMode_STOP_MODE_GRACEFUL
		if force {
			stopMode = pb.StopMode_STOP_MODE_IMMEDIATE
		}

		_, err = cl.Stop(context.Background(), &pb.StopRequest{
			JobId: args[0],
			Mode:  stopMode,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to stop the job: %v\n", diagMessage(err))
			os.Exit(1)
		}
	},
}

var force bool

func init() {
	stopCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "Force stop the job")
	rootCmd.AddCommand(stopCmd)
}
