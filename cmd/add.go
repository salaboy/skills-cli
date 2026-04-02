package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-cli/pkg/oci"
	"github.com/salaboy/skills-cli/pkg/tui/add"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Install a skill from an OCI registry",
		Long:  "Pulls a skill artifact from a remote container registry and extracts it to the local .agents/skills directory.",
		Example: `  # Install a skill from GHCR
  skills add --ref ghcr.io/myorg/skills/my-skill:1.0.0

  # Install from a local registry
  skills add --ref localhost:5000/my-skill:1.0.0 --plain-http

  # Install to a custom directory
  skills add --ref ghcr.io/myorg/skills/my-skill:1.0.0 --output ./custom/path`,
		RunE: runAdd,
	}

	cmd.Flags().String("ref", "", "Full OCI reference (e.g., ghcr.io/org/skills/my-skill:1.0.0)")
	cmd.Flags().String("output", "", "Output directory (default: .agents/skills)")

	_ = cmd.MarkFlagRequired("ref")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	ref, _ := cmd.Flags().GetString("ref")
	output, _ := cmd.Flags().GetString("output")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runAddPlain(ref, output, plainHTTP)
	}

	m := add.NewModel(ref, output, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	if fm, ok := finalModel.(add.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runAddPlain(ref, output string, plainHTTP bool) error {
	result, err := oci.Pull(context.Background(), oci.PullOptions{
		Reference: ref,
		OutputDir: output,
		PlainHTTP: plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nSuccessfully installed!\n")
	fmt.Printf("  Name:      %s\n", result.Name)
	fmt.Printf("  Version:   %s\n", result.Version)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Extracted: %s\n", result.ExtractTo)
	return nil
}
