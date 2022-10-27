package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	pb "github.com/ilyazz/jobs/pkg/api/grpc/jobs/v1"
	"github.com/ilyazz/jobs/pkg/client"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Get job output",
	Long:  `TODO`,
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

		rsp, err := cl.Logs(context.Background(), &pb.LogsRequest{
			JobId: args[0],
			Options: &pb.LogsOptions{
				Follow: follow,
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to get logs: %v\n", diagMessage(err))
			os.Exit(1)
		}

		var out pb.LogsResponse
		for {
			err = rsp.RecvMsg(&out)
			if err != nil {
				if err == io.EOF {
					return
				}
				fmt.Printf("failed to read logs: %s", diagMessage(err))
				os.Exit(1)
			}
			fmt.Print(string(out.Data))
		}
	},
}

var follow bool

func init() {
	logsCmd.PersistentFlags().BoolVar(&follow, "f", false, "follow mode")

	rootCmd.AddCommand(logsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// logsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// logsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
