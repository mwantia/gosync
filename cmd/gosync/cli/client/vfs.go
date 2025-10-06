package client

import "github.com/spf13/cobra"

func NewVfsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vfs",
		Short: "Manage virtual filesystem",
		Long:  "Manage the virtual filesystem (VFS) and list, create or update entries.",
	}

	cmd.AddCommand(NewVfsListCommand())
	cmd.AddCommand(NewVfsTestCommand())
	cmd.AddCommand(NewVfsTouchCommand())
	cmd.AddCommand(NewVfsRemoveCommand())
	cmd.AddCommand(NewVfsCreateDirectoryCommand())

	return cmd
}

func NewVfsListCommand() *cobra.Command {
	var humanReadable bool
	var longFormat bool

	cmd := &cobra.Command{
		Use:   "ls [path]",
		Short: "List virtual filesystem entries",
		Long:  "List all entries existing within the defined virtual filesystem path.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().BoolVarP(&humanReadable, "human", "H", false, "Enable human-readable format")
	cmd.Flags().BoolVarP(&longFormat, "long", "l", false, "Display long format")

	return cmd
}

func NewVfsTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <path>",
		Short: "Test virtual filesystem",
		Long:  "Tests if the defined path exists within the virtual filesystem.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	return cmd
}

func NewVfsTouchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "touch <path>",
		Short: "Update virtual filesystem metadata",
		Long:  "Updates the virtual filesystem metadata of an entry or creates it if it doesn't already exist.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	return cmd
}

func NewVfsRemoveCommand() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "rm <path>",
		Short: "Removes virtual filesystem entry",
		Long:  "Removes the virtual filesystem entry defined in the path. Can also be used to wipe a backend (needs confirmation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	cmd.Flags().BoolVarP(&confirm, "confirm", "c", false, "Confirms the deletion of a backend")

	return cmd
}

func NewVfsCreateDirectoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mkdir <path>",
		Short: "Create virtual filesystem prefix",
		Long:  "Create a new virtual filesystem prefix within in the defined path.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	return cmd
}
