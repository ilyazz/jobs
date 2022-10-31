package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/ilyazz/jobs/pkg/client"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Create or update config file",
	Long:  `Create or update config file. TODO: add more details`,
	Run: func(cmd *cobra.Command, args []string) {

		file, cfg, err := client.FindConfig(config)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			os.Exit(1)
		}

		if err := client.SaveConfig(file, cfg); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to save config file: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
