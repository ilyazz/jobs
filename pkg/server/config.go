package server

import (
	"flag"
	"os"
	"os/user"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the server config
type Config struct {
	// root dir for all job directories
	WorkRoot string `mapstructure:"workroot"`
	// Superusers is a simple list of user ids, who have access to all jobs
	Superusers struct {
		FullAccess []string `mapstructure:"full"`
		ReadAccess []string `mapstructure:"read"`
	} `mapstructure:"superusers"`
	// TLS is a set of TLS paramaters
	TLS struct {
		// CAPath is a path to CA cert to verify clients
		CAPath string `mapstructure:"capath"`
		// CertPath is the path where to find the server cert
		CertPath string `mapstructure:"cert"`
		// CertPKeyPathath is the path where to find the server cert key
		KeyPath string `mapstructure:"key"`
		// ReloadSec is the interval in seconds to reload server cert/key pair
		ReloadSec int `mapstructure:"reloadSec"`
	} `mapstructure:"tls"`
	// IDs is the uid/gid to run job processes with
	IDs struct {
		// UID is user id.
		UID string `mapstructure:"uid"`
		// GID is group id
		GID string `mapstructure:"gid"`
	} `mapstructure:"ids"`
	// Address is the server address
	Address string `mapstructure:"address"`
}

// FindConfig ties to find server config
func FindConfig(file string) (*Config, error) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	_ = viper.BindPFlags(pflag.CommandLine)

	viper.AddConfigPath(".")
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
	}

	viper.AddConfigPath("/etc")
	viper.SetConfigName(file)
	viper.SetConfigType("yaml")

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	var conf Config
	err = viper.Unmarshal(&conf)
	if err != nil {
		return nil, err
	}

	if conf.TLS.ReloadSec <= 0 {
		conf.TLS.ReloadSec = 30
	}

	if f := pflag.Lookup("uid"); f != nil {
		conf.IDs.UID = f.Value.String()
	}

	if f := pflag.Lookup("gid"); f != nil {
		conf.IDs.GID = f.Value.String()
	}

	u, err := user.Lookup(conf.IDs.UID)
	if err == nil {
		conf.IDs.UID = u.Uid
	}

	g, err := user.LookupGroup(conf.IDs.GID)
	if err == nil {
		conf.IDs.GID = g.Gid
	}

	return &conf, nil
}
