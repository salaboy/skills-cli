package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/salaboy/skills-cli/pkg/skill"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove an installed skill",
		Long:  "Removes a skill from .agents/skills and updates skills.json and skills.lock.json.",
		Example: `  # Remove a skill by name
  skills remove --name manage-pull-requests`,
		RunE: runRemove,
	}

	cmd.Flags().String("name", "", "Name of the skill to remove")
	cmd.Flags().String("project-dir", ".", "Project directory containing skills.json and skills.lock.json")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func runRemove(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	projectDir, _ := cmd.Flags().GetString("project-dir")

	// Load and update skills.json
	m, err := skill.LoadManifest(projectDir)
	if err != nil {
		return fmt.Errorf("loading skills.json: %w", err)
	}

	if !skill.RemoveFromManifest(m, name) {
		return fmt.Errorf("skill %q not found in skills.json", name)
	}

	if err := skill.SaveManifest(projectDir, m); err != nil {
		return fmt.Errorf("saving skills.json: %w", err)
	}
	fmt.Println("  Updated skills.json")

	// Load and update skills.lock.json
	l, err := skill.LoadLock(projectDir)
	if err != nil {
		return fmt.Errorf("loading skills.lock.json: %w", err)
	}

	skill.RemoveFromLock(l, name)

	if err := skill.SaveLock(projectDir, l); err != nil {
		return fmt.Errorf("saving skills.lock.json: %w", err)
	}
	fmt.Println("  Updated skills.lock.json")

	// Remove the extracted skill directory
	skillDir := filepath.Join(projectDir, ".agents", "skills", name)
	if err := os.RemoveAll(skillDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing skill directory: %w", err)
	}
	fmt.Printf("  Removed %s\n", skillDir)

	fmt.Printf("\nSuccessfully removed skill %q\n", name)
	return nil
}
