// Copyright (c) Elliot Peele <elliot@bentlogic.net>

package cmd

import (
	"os"

	"github.com/elliotpeele/sshhttpproxy/proxy"
	"github.com/spf13/viper"
)

// ProxyFromConfig creates a proxy instance based on config file content.
func ProxyFromConfig() (*proxy.SSHProxy, error) {
	cfg := &proxy.Config{
		PrivateKeyPath: os.ExpandEnv(viper.GetString("sshproxy.privatekey")),
		RemoteUser:     viper.GetString("sshproxy.user"),
		RemoteAddress:  viper.GetString("sshproxy.remote"),
	}
	return proxy.New(cfg)
}
