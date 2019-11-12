// Copyright © 2019 Elliot Peele <elliot@bentlogic.net>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	logging "github.com/op/go-logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var logger = logging.MustGetLogger("sshhttpproxy.cmd")

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "sshhttpproxy",
	Short: "An http proxy over ssh tunnel",
	Long: `Port forward HTTP connections over an SSH tunnel automatically using the
HTTP proxy protocol`,
	RunE: func(cmd *cobra.Command, args []string) error {
		debug, _ := cmd.InheritedFlags().GetBool("debug")
		setupLogging(os.Stderr, debug)
		logger.Debugf("debug logging enabled")
		ctx, cancel := context.WithCancel(context.Background())
		go setupSignalHandler(ctx, cancel)
		defer cancel()
		remotes, err := cmd.PersistentFlags().GetStringSlice("remote")
		if err != nil {
			return err
		}
		localPort, err := cmd.PersistentFlags().GetString("local")
		if err != nil {
			return err
		}
		p, err := ProxyFromConfig()
		if err != nil {
			return err
		}
		p.WithContext(ctx)
		logger.Infof("connecting to %s@%s",
			viper.GetString("sshproxy.user"),
			viper.GetString("sshproxy.remote"))
		if err := p.Connect(); err != nil {
			return err
		}
		for _, remote := range remotes {
			local, err := p.Forward(remote, localPort)
			if err != nil {
				return err
			}
			logger.Infof("%s -> %s", remote, local)
		}
		// TODO: wait for ctl-c and shutdown
		if ctx.Err() == context.Canceled {
			fmt.Fprintln(os.Stderr, "Mirror interrupted by signal")
			os.Exit(1)
		}
		select {}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sshhttpproxy.yaml)")
	rootCmd.Flags().BoolP("debug", "d", false, "enable debug level logging")
	rootCmd.PersistentFlags().StringSliceP("remote", "r", nil, "remote server and port")
	rootCmd.PersistentFlags().String("local", "0", "set local port")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".sshhttpproxy" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".sshhttpproxy")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func setupLogging(out io.Writer, debug bool) {
	backend := logging.AddModuleLevel(
		logging.NewBackendFormatter(
			logging.NewLogBackend(out, "", 0),
			logging.MustStringFormatter("%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.8s} %{id:03x}%{color:reset} %{message}"),
		),
	)
	if debug {
		logging.SetLevel(logging.DEBUG, "")
	} else {
		logging.SetLevel(logging.INFO, "")
	}
	logging.SetBackend(backend)
}
