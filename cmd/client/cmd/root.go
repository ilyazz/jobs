package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jctrl",
	Short: "Job service client",
	Long:  `Job service client`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var config string
var caPath string
var cert string
var key string
var server string

func init() {
	rootCmd.PersistentFlags().StringVar(&config, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&caPath, "capath", "", "ca cert file path")
	rootCmd.PersistentFlags().StringVar(&cert, "cert", "", "tls client cert file path")
	rootCmd.PersistentFlags().StringVar(&key, "key", "", "tls client key file path")
	rootCmd.PersistentFlags().StringVar(&server, "server", "", "job server address")

	err := viper.BindPFlags(rootCmd.PersistentFlags())

	if err != nil {
		panic(err)
	}

}
