package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-cli/pkg/oci"
	"github.com/salaboy/skills-cli/pkg/tui/push"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Package and push a skill to an OCI registry",
		Long:  "Validates a skill directory, packages it as an OCI artifact, and pushes it to a remote container registry.",
		Example: `  # Push a skill to GHCR
  skills push --ref ghcr.io/myorg/skills/my-skill --path ./my-skill --tag 1.0.0

  # Push to a local registry (plain HTTP)
  skills push --ref localhost:5000/my-skill --path ./my-skill --tag 1.0.0 --plain-http`,
		RunE: runPush,
	}

	cmd.Flags().String("ref", "", "Registry reference (e.g., ghcr.io/org/skills/my-skill)")
	cmd.Flags().String("path", ".", "Path to skill directory")
	cmd.Flags().String("tag", "", "Version tag (e.g., 1.0.0)")

	_ = cmd.MarkFlagRequired("ref")

	return cmd
}

func runPush(cmd *cobra.Command, args []string) error {
	ref, _ := cmd.Flags().GetString("ref")
	path, _ := cmd.Flags().GetString("path")
	tag, _ := cmd.Flags().GetString("tag")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runPushPlain(ref, path, tag, plainHTTP)
	}

	m := push.NewModel(ref, tag, path, plainHTTP)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check if the final model has an error
	if fm, ok := finalModel.(push.Model); ok {
		if fm.Err() != nil {
			return fm.Err()
		}
	}

	return nil
}

func runPushPlain(ref, path, tag string, plainHTTP bool) error {
	result, err := oci.Push(context.Background(), oci.PushOptions{
		Reference: ref,
		Tag:       tag,
		SkillDir:  path,
		PlainHTTP: plainHTTP,
		OnStatus: func(phase string) {
			fmt.Printf("  %s\n", phase)
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nSuccessfully pushed!\n")
	fmt.Printf("  Reference: %s:%s\n", result.Reference, result.Tag)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Size:      %d bytes\n", result.Size)
	return nil
}
