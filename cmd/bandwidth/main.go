// Command bandwidth runs the BandWidth server.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jwhumphries/bandwidth/version"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	initConfig()
	return &cobra.Command{
		Use:           "bandwidth",
		Short:         "Practice tracking for musicians and bands",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			return runServer()
		},
	}
}

// initConfig wires Viper to BANDWIDTH_* environment variables.
// Keys: port (BANDWIDTH_PORT), log_level (BANDWIDTH_LOG_LEVEL).
func initConfig() {
	viper.SetDefault("port", ":8080")
	viper.SetDefault("log_level", "info")
	viper.SetEnvPrefix("BANDWIDTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}
