package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCmd creates the root command for skills-cli.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage agent skills as OCI artifacts",
		Long:  "A CLI tool for packaging, pushing, and pulling agent skills as OCI artifacts following the Agent Skills OCI Artifacts Specification.",
	}

	cmd.PersistentFlags().Bool("plain", false, "Disable interactive TUI (plain text output)")
	cmd.PersistentFlags().Bool("plain-http", false, "Use plain HTTP instead of HTTPS for registry connections")

	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newAddCmd())

	return cmd
}
