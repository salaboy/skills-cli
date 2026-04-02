package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/salaboy/skills-cli/pkg/oci"
	"github.com/salaboy/skills-cli/pkg/skill"
	"github.com/salaboy/skills-cli/pkg/tui/add"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Install a skill from an OCI registry",
		Long:  "Pulls a skill artifact from a remote container registry, extracts it to .agents/skills, and updates skills.json and skills.lock.json.",
		Example: `  # Install a skill from GHCR
  skills add --ref ghcr.io/myorg/skills/my-skill:1.0.0

  # Install from a local registry
  skills add --ref localhost:5000/my-skill:1.0.0 --plain-http

  # Install to a custom directory
  skills add --ref ghcr.io/myorg/skills/my-skill:1.0.0 --output ./custom/path`,
		RunE: runAdd,
	}

	cmd.Flags().String("ref", "", "Full OCI reference (e.g., ghcr.io/org/skills/my-skill:1.0.0)")
	cmd.Flags().String("output", "", "Output directory for skill extraction (default: .agents/skills)")
	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")

	_ = cmd.MarkFlagRequired("ref")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	ref, _ := cmd.Flags().GetString("ref")
	output, _ := cmd.Flags().GetString("output")
	projectDir, _ := cmd.Flags().GetString("project-dir")
	plain, _ := cmd.Flags().GetBool("plain")
	plainHTTP, _ := cmd.Flags().GetBool("plain-http")

	if plain {
		return runAddPlain(ref, output, projectDir, plainHTTP)
	}

	m := add.NewModel(ref, output, projectDir, plainHTTP)
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

func runAddPlain(ref, output, projectDir string, plainHTTP bool) error {
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

	// Update skills.json
	fmt.Println("  Updating skills.json")
	if err := updateManifest(projectDir, result); err != nil {
		return fmt.Errorf("updating skills.json: %w", err)
	}

	// Update skills.lock.json
	fmt.Println("  Updating skills.lock.json")
	if err := updateLockFile(projectDir, result); err != nil {
		return fmt.Errorf("updating skills.lock.json: %w", err)
	}

	fmt.Printf("\nSuccessfully installed!\n")
	fmt.Printf("  Name:      %s\n", result.Name)
	fmt.Printf("  Version:   %s\n", result.Version)
	fmt.Printf("  Digest:    %s\n", result.Digest)
	fmt.Printf("  Extracted: %s\n", result.ExtractTo)
	return nil
}

// updateManifest loads skills.json, adds/updates the skill entry, and saves it.
func updateManifest(projectDir string, result *oci.PullResult) error {
	m, err := skill.LoadManifest(projectDir)
	if err != nil {
		return err
	}

	skill.AddToManifest(m, result.Name, result.Source(), result.Version)

	return skill.SaveManifest(projectDir, m)
}

// updateLockFile loads skills.lock.json, adds/updates the skill entry, and saves it.
func updateLockFile(projectDir string, result *oci.PullResult) error {
	l, err := skill.LoadLock(projectDir)
	if err != nil {
		return err
	}

	// Compute relative path from project dir to extracted skill
	extractPath := filepath.Join(".agents", "skills", result.Name)

	entry := skill.LockedSkill{
		Name: result.Name,
		Path: extractPath,
		Source: skill.LockSource{
			Registry:   result.Registry,
			Repository: result.Repository,
			Tag:        result.Tag,
			Digest:     result.Digest,
			Ref:        result.FullRef(),
		},
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}

	skill.AddToLock(l, entry)

	return skill.SaveLock(projectDir, l)
}
