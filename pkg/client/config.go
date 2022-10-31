package client

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Config is the set of client options
type Config struct {
	// Server is the job server endpoint
	Server string `mapstructure:"server"`
	// CAPath is the paht to CA cert to verify server
	CAPath string `mapstructure:"capath"`
	// CertPath is the path to the client cert file
	CertPath string `mapstructure:"cert" yaml:"cert"`
	// KeyPath is the path to the client cert key file
	KeyPath string `mapstructure:"key" yaml:"key"`
}

// FindConfig tries to find a suitable config file,
// returning a path to the used file, config itself and error if any
func FindConfig(file string) (string, *Config, error) {
	viper.AddConfigPath(".")

	defConfig := "jctrl.yaml"

	if file != "" {
		viper.SetConfigFile(file)
	} else {
		viper.AddConfigPath("/etc")
		viper.SetConfigName(defConfig)
		viper.SetConfigType("yaml")

		file = defConfig

		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.AddConfigPath(filepath.Join(home, ".jobs"))
		}
	}

	var c Config

	if err := viper.ReadInConfig(); err != nil {
		err = viper.Unmarshal(&c)
		if err == nil {
			return file, &c, nil
		}
		return file, nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := viper.Unmarshal(&c); err != nil {
		return "", &c, fmt.Errorf("failed to parse config: %w", err)
	}

	return viper.ConfigFileUsed(), &c, nil
}

// SaveConfig stores the config object to file
func SaveConfig(file string, cfg *Config) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	defer func() { _ = f.Close() }()

	enc := yaml.NewEncoder(f)

	return enc.Encode(cfg)
}
