package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	config "github.com/mwantia/gosync/internal/config/server"
)

func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management utilities",
		Long: `Manage GoSync Client Agent configuration files.

This command provides utilities for generating, validating, and 
managing configuration files for different environments.`,
	}

	cmd.AddCommand(newConfigGenerateCommand())

	return cmd
}

func newConfigGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate example configuration files",
		Long: `Generate example configuration files for different environments.

This command creates configuration templates that can be customized
for your specific deployment requirements.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir, _ := cmd.Flags().GetString("output")
			overwrite, _ := cmd.Flags().GetBool("overwrite")

			fmt.Printf("Generating configuration files (output: %s)\n", outputDir)

			// Create output directory if it doesn't exist
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			filename := filepath.Join(outputDir, "gosync.yaml")

			// Check if file exists and overwrite flag
			if _, err := os.Stat(filename); err == nil && !overwrite {
				fmt.Printf("Skipping %s (file exists, use --overwrite to replace)\n", filename)
			}

			cfg := config.GetServerDefault()
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal config: %w", err)
			}

			if err := os.WriteFile(filename, data, 0644); err != nil {
				return fmt.Errorf("failed to write config file %s: %w", filename, err)
			}

			fmt.Printf("Generated %s\n", filename)

			fmt.Println("Configuration generation complete!")
			return nil
		},
	}

	cmd.Flags().String("output", ".", "output directory for configuration files")
	cmd.Flags().Bool("overwrite", false, "overwrite existing files")

	return cmd
}
