package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/client"
)

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove stopped job files",
	Long:  `Remove stopped job files. TODO: add more details here`,
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

		_, err = cl.Remove(context.Background(), &pb.RemoveRequest{
			JobId: args[0],
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to stop the job: %v\n", diagMessage(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
