package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/client"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Starts a new remote job",
	Long:  `TODO: add more details here`,
	Run: func(cmd *cobra.Command, args []string) {
		_, cfg, err := client.FindConfig(config)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to load config\n")
			os.Exit(1)
		}

		if len(args) == 0 {
			_, _ = fmt.Fprintf(os.Stderr, "command required\n")
			os.Exit(1)
		}

		cl, err := client.New(cfg)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
			os.Exit(1)
		}

		rsp, err := cl.Start(context.Background(), &pb.StartRequest{
			Command: args[0],
			Args:    args[1:],
			Limits: &pb.Limits{
				Cpus:   cpuLimit,
				Memory: memLimit,
				Io:     ioLimit,
			},
		})

		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to start the job: %v\n", diagMessage(err))
			os.Exit(1)
		}

		_, _ = fmt.Print(rsp.JobId)
	},
}

var cpuLimit float32
var memLimit int64
var ioLimit int64

func init() {

	runCmd.PersistentFlags().Float32Var(&cpuLimit, "cpu", 1.0, "CPU limit for the job. No limit if zero or not set.")
	runCmd.PersistentFlags().Int64Var(&memLimit, "mem", 0, "RAM limit for the job. No limit if zero or not set.")
	runCmd.PersistentFlags().Int64Var(&ioLimit, "io", 0, "IO rate limit for the job. No limit if zero or not set.")

	rootCmd.AddCommand(runCmd)
}
