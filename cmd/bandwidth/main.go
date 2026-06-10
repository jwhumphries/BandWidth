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
	return &cobra.Command{
		Use:           "bandwidth",
		Short:         "Practice tracking for musicians and bands",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			initConfig()
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			return runServer()
		},
	}
}

// initConfig wires Viper to BANDWIDTH_* environment variables.
// Keys: port, log_level, db_path.
func initConfig() {
	viper.SetDefault("port", ":8080")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("db_path", "data/bandwidth.db")
	viper.SetEnvPrefix("BANDWIDTH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}
