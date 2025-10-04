package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewRootCommand(info VersionInfo) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:           "gosync",
		Short:         "GoSync Sync S3 Client",
		Long:          "A production-ready, cross-platform sync client for S3 (MinIO) that provides true bidirectional synchronization similar to MegaSync, Dropbox, or OneDrive.",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig(path)
		},
	}

	cmd.PersistentFlags().StringVar(&path, "config", "", "config file (default is ./config.yaml)")
	cmd.PersistentFlags().Bool("no-color", false, "Disables colored command output")
	cmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")

	viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("log.no_color", cmd.PersistentFlags().Lookup("no-color"))

	cmd.Version = fmt.Sprintf("%s.%s", info.Version, info.Commit)

	return cmd
}
