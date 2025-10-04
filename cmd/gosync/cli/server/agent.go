package server

import (
	"context"
	"fmt"

	"github.com/mwantia/gosync/internal/agent"
	"github.com/spf13/cobra"

	config "github.com/mwantia/gosync/internal/config/server"
)

func NewAgentCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start the GoSync Client Agent",
		Long:  `Start the GoSync Client Agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadServerConfig()
			if err != nil {
				return fmt.Errorf("failed to load server configuration: %w", err)
			}

			agent := agent.NewAgent(cfg)
			if err := agent.Serve(context.Background()); err != nil {
				print(err)
				return err
			}

			return nil
		},
	}

	return cmd
}
