package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)

// Validate checks that the given path is a valid skill directory.
func Validate(skillPath string) error {
	info, err := os.Stat(skillPath)
	if err != nil {
		return fmt.Errorf("skill path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill path is not a directory: %s", skillPath)
	}

	skillMD := filepath.Join(skillPath, "SKILL.md")
	if _, err := os.Stat(skillMD); err != nil {
		return fmt.Errorf("skill directory must contain SKILL.md: %w", err)
	}

	return nil
}

// ValidateName checks that a skill name matches the spec pattern.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("skill name %q does not match required pattern %s (2-64 lowercase alphanumeric chars and hyphens)", name, namePattern.String())
	}
	return nil
}
